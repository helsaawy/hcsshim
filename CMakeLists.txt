cmake_minimum_required(VERSION 3.25)

project(hcsshim
    DESCRIPTION "project contains Golang interface for the Windows Host Compute Service and OCI shim used by containerd"
    LANGUAGES NONE
)

cmake_path(SET CMAKE_MODULE_PATH ${CMAKE_SOURCE_DIR}/cmake)
find_package(Go 1.18 REQUIRED
    COMPONENTS Linter
)

# TODO:
#
# common go env variable (GOTOOLCHAIN, GOWORK, GO111MODULE, etc.)
#
# debug vs release build variant: set version, trimpath, symbols, etc...
#
# use ctest to run `gotestsum`
#
#  move commands (CPLat and into active VM)
#  - restart containerd service before hand
#
#  build linux files
#  - start with wsl
#  - tar-merge to combine tar layers with crane mutate (for LCOW rootfs)
#  - tar-to-cpio to convert tar to cpio (with hardlink information)
#    - https://microsoft.visualstudio.com/HyperVCloud/_git/hvlite?path=/ci/gen_init_ramfs.py

# for downloading (mostly go) archives
set(arch amd64)
set(archive_ext .tar)
set(os linux)
if(WIN32)
    set(archive_ext .zip)
    set(binary_ext .exe)
    set(os windows)
endif()

set(go_common_env GOTOOLCHAIN=local GO111MODULE=on GOWORK=off)

# todo: consolidate these and move them under CMAKE_BUILD_DIR
cmake_path(SET BIN_DIR ${CMAKE_SOURCE_DIR}/bin)
cmake_path(SET TEST_BIN ${BIN_DIR}/test) # testing binaries
cmake_path(SET TOOL_BIN ${BIN_DIR}/tool) # ancillary tools (eg, gotestsum, benchstat, crane)

cmake_path(SET DEP_DIR ${CMAKE_SOURCE_DIR}/deps)
cmake_path(SET OUT_DIR ${CMAKE_SOURCE_DIR}/out)
cmake_path(SET PROTOBUF_DIR ${CMAKE_SOURCE_DIR}/protobuf)
cmake_path(SET TEST_DIR ${CMAKE_SOURCE_DIR}/test)

cmake_path(SET CMAKE_RUNTIME_OUTPUT_DIRECTORY ${BIN_DIR})
cmake_path(SET CMAKE_ARCHIVE_OUTPUT_DIRECTORY ${OUT_DIR})

file(MAKE_DIRECTORY ${TEST_BIN} ${TOOL_BIN} ${DEP_DIR} ${OUT_DIR} ${PROTOBUF_DIR})

if(Go_GOBIN)
    cmake_path(SET go_bin Go_GOBIN)
elseif(Go_GOPATH)
    cmake_path(SET go_bin "${Go_GOPATH}/bin")
else()
    cmake_path(SET go_bin "${TOOL_BIN}")
endif()
message(STATUS "go_bin: ${go_bin}")

##########################################################################################
# install go dependencies
##########################################################################################

foreach (i IN ITEMS
    "gotest.tools/gotestsum@latest"
    "golang.org/x/perf/cmd/benchstat@latest"
    "github.com/google/go-containerregistry/cmd/crane@latest"
)
    string(REGEX REPLACE [[@.*]] "" url ${i})
    cmake_path(GET url FILENAME binary)

    find_program("${binary}_loc" ${binary} HINTS ${OUT_DIR} ${go_bin} NO_CACHE)
    if (EXISTS ${${binary}_loc})
        add_executable(${binary} IMPORTED GLOBAL)
        set_target_properties(${binary} PROPERTIES IMPORTED_LOCATION ${${binary}_loc})
        message(VERBOSE "found ${binary} - ${${binary}_loc}")
        continue()
    endif()

    message(STATUS "go install ${binary} - ${i}")

    # custom targets are alway considered out-of-date, so use a custom command to avoid re-installing
    cmake_path(SET "${binary}_loc" ${go_bin}/${binary}${binary_ext})
    set(env GOTOOLCHAIN=local GOBIN=${go_bin})
    add_custom_command(OUTPUT "${${binary}_loc}"
        COMMAND ${CMAKE_COMMAND} -E env ${env} -- ${Go_EXECUTABLE} install ${i}
        COMMENT "installing ${binary} from ${i}"
        VERBATIM
    )
    add_custom_target(${binary} DEPENDS "${${binary}_loc}")
    set_target_properties(${binary} PROPERTIES FOLDER dependencies/go)
endforeach()

##########################################################################################
# go mod tidy and vendor
##########################################################################################

add_custom_target(tidy ALL)
set_target_properties(tidy PROPERTIES FOLDER validation)

function(go_tidy name vendor)
    set(flags "-e")

    # cmake automatically creates a `${CMAKE_BINARY_DIR}/CMakeFiles/${name}` output for the custom target
    set(phony ${CMAKE_BINARY_DIR}/CMakeFiles/go_tidy_${name})
    add_custom_target(${name} DEPENDS ${phony})

    add_custom_command(OUTPUT ${phony}
        COMMAND ${CMAKE_COMMAND} -E env ${go_common_env} -- ${Go_EXECUTABLE} mod tidy -v ${flags}
        COMMENT "tidying go module ${CMAKE_CURRENT_SOURCE_DIR}"
        WORKING_DIRECTORY ${CMAKE_CURRENT_SOURCE_DIR}
        USES_TERMINAL
    )
    if (vendor)
        add_custom_command(OUTPUT ${phony}
            COMMAND ${CMAKE_COMMAND} -E env ${go_common_env} -- ${Go_EXECUTABLE} mod vendor ${flags}
            APPEND
        )
    endif()

    add_dependencies(tidy ${name})
    set_target_properties(${name} PROPERTIES FOLDER validation)
    message(STATUS "go mod tidy ${name}")
endfunction()

go_tidy(tidy-root true)

##########################################################################################
# go fmt
##########################################################################################

# TODO: make this a command so we can gofmt the test directory
# running this affects the schema files, so don't add as a dependency for now
add_custom_target(gofmt
    COMMAND ${CMAKE_COMMAND} -E env ${go_common_env} GOOS=windows -- ${Go_EXECUTABLE} fmt ./...
    COMMAND ${CMAKE_COMMAND} -E env ${go_common_env} GOOS=linux -- ${Go_EXECUTABLE} fmt ./...
    COMMENT "running 'go fmt'"
    WORKING_DIRECTORY ${CMAKE_CURRENT_SOURCE_DIR}
    USES_TERMINAL
)
set_target_properties(gofmt PROPERTIES FOLDER validation)

##########################################################################################
# go generate
##########################################################################################

add_custom_target(gen
    COMMAND ${CMAKE_COMMAND} -E env ${go_common_env} -- ${Go_EXECUTABLE} generate -x ./...
    DEPENDS tidy-root
    COMMENT "running 'go generate'"
    WORKING_DIRECTORY ${CMAKE_CURRENT_SOURCE_DIR}
    USES_TERMINAL
)
set_target_properties(gen PROPERTIES FOLDER generation)

##########################################################################################
# build binaries
##########################################################################################

add_custom_target(set_version
    COMMAND pwsh -NonInteractive -NoProfile -NoLogo -Command "./scripts/Set-VersionInfo.ps1"
    DEPENDS tidy-root
    COMMENT "run Set-Version script"
    WORKING_DIRECTORY ${CMAKE_SOURCE_DIR}
)

add_custom_target(build
    COMMENT "build all project go binaries"
)
set_target_properties(build PROPERTIES FOLDER binaries)

# go_build <package> <GOOS> [target_name]
function(go_build package os)
    cmake_path(GET package FILENAME binary)
    cmake_path(SET package ${CMAKE_SOURCE_DIR}/${package})
    cmake_path(SET out ${BIN_DIR}/${binary})
    set(flags -trimpath "-ldflags=-s -w")

    if (os STREQUAL windows)
        cmake_path(APPEND_STRING out binary_ext)
    endif()

    set(name ${binary})
    if ( ARGC GREATER 2 )
        set(name ${ARGV2})
    endif()

    add_custom_target(${name}
        BYPRODUCTS ${out}
        COMMAND ${CMAKE_COMMAND} -E env ${go_common_env} CGO_ENABLED=0 GOOS=${os} -- ${Go_EXECUTABLE} build ${flags} -o=${out}
        DEPENDS set_version tidy-root
        COMMENT "building go binary ${binary} for ${os}"
        WORKING_DIRECTORY "${package}"
    )
    add_dependencies(build ${name})
    set_target_properties(${name} PROPERTIES FOLDER binaries)

    message(STATUS "go binary ${name} - ${out}")
endfunction()

go_build(cmd/containerd-shim-runhcs-v1 windows shim)
go_build(cmd/device-util windows)
go_build(cmd/dmverity-vhd windows)
go_build(cmd/gcs linux)
go_build(cmd/gcstools linux)
go_build(cmd/hooks/wait-paths linux)
go_build(cmd/ncproxy windows)
go_build(cmd/runhcs windows)
go_build(cmd/shimdiag windows)
go_build(cmd/tar2ext4 windows)
go_build(cmd/tar2ext4 linux tar2ext4-linux)
go_build(cmd/wclayer windows)

go_build(internal/tools/extendedtask windows)
go_build(internal/tools/grantvmgroupaccess windows vmaccess)
go_build(internal/tools/networkagent windows)
go_build(internal/tools/policyenginesimulator linux)
go_build(internal/tools/securitypolicy linux)
go_build(internal/tools/snp-report linux)
go_build(internal/tools/uvmboot windows)
go_build(internal/tools/zapdir windows)

##########################################################################################
# go tests
##########################################################################################

add_custom_target(test
    COMMAND ${CMAKE_COMMAND} -E env ${go_common_env} -- ${gotestsum_loc} --format=testname -- -tags=admin "-gcflags=all=-d=checkptr" -v -timeout=30m -race ./...
    DEPENDS gotestsum
    COMMENT "running go tests"
    WORKING_DIRECTORY ${CMAKE_SOURCE_DIR}
    USES_TERMINAL
    VERBATIM
)
add_custom_target(test_hcn
    COMMAND ${CMAKE_COMMAND} -E env ${go_common_env} -- ${gotestsum_loc} --format=testname -- -tags=admin -tags=integration "-gcflags=all=-d=checkptr" -v -timeout=30m -race hcn/...
    DEPENDS gotestsum
    COMMENT "running go tests in ./hcn"
    WORKING_DIRECTORY ${CMAKE_SOURCE_DIR}
    USES_TERMINAL
    VERBATIM
)
add_custom_target(test_all DEPENDS test test_hcn)

##########################################################################################
# lint
##########################################################################################

add_custom_target(lint)
set_target_properties(lint PROPERTIES FOLDER validation)

function(lint name os)
    set(flags --timeout=2m --max-issues-per-linter=0 --max-same-issues=0 --modules-download-mode=readonly --verbose --config=${CMAKE_SOURCE_DIR}/.golangci.yml)

    add_custom_target(${name}
        COMMAND ${CMAKE_COMMAND} -E env ${go_common_env} GOOS=${os} -- ${Go_Linter_EXECUTABLE} run ${flags} ${ARGN}
        DEPENDS tidy
        COMMENT "linting ${CMAKE_CURRENT_SOURCE_DIR} for ${os}"
        WORKING_DIRECTORY ${CMAKE_CURRENT_SOURCE_DIR}
        USES_TERMINAL
    )
    add_dependencies(lint ${name})
    set_target_properties(${name} PROPERTIES FOLDER validation)
    message(STATUS "golangci-lint ${name}")
endfunction()

lint(lint-root windows)
lint(lint-root-linux linux)

##########################################################################################
# protobuf
##########################################################################################

add_custom_target(proto
    COMMAND ${CMAKE_COMMAND} -E env ${go_common_env} -- powershell.exe -Command "& \"${Go_EXECUTABLE}\" list ./... | Where-Object { \$_ -notlike \"*/vendor/*\" } | ForEach-Object { & \"${protobuild_loc}\" \$_ }"
    DEPENDS protobuild protoc-gen-gogoctrd
    COMMENT building protofiles
    WORKING_DIRECTORY ${CMAKE_SOURCE_DIR}
    VERBATIM
)
set_target_properties(proto PROPERTIES FOLDER generation)

##########################################################################################
# pr validation
##########################################################################################

add_custom_target(validate DEPENDS tidy lint gen)
set_target_properties(validate PROPERTIES FOLDER validation)

##########################################################################################
# sub-modules
#
# add additional modules after all the custom commands and functions have been defined
##########################################################################################
add_subdirectory(test)
