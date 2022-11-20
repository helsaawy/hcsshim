cmake_minimum_required(VERSION 3.25)

project(hcsshim
    DESCRIPTION "project contains Golang interface for the Windows Host Compute Service and OCI shim used by containerd"
    LANGUAGES NONE
)

cmake_path(SET CMAKE_MODULE_PATH ${CMAKE_SOURCE_DIR}/cmake)
find_package(Go 1.17 REQUIRED
    COMPONENTS Linter
)

# TODO:
#
# `-trimpath` for building
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

# todo: consolidate these and move them under CMAKE_BUILD_DIR
cmake_path(SET BIN_DIR ${CMAKE_SOURCE_DIR}/bin)
cmake_path(SET CMD_BIN ${BIN_DIR}/cmd)
cmake_path(SET TEST_BIN ${BIN_DIR}/test)
cmake_path(SET TOOL_BIN ${BIN_DIR}/tool)

cmake_path(SET DEP_DIR ${CMAKE_SOURCE_DIR}/deps)
cmake_path(SET OUT_DIR ${CMAKE_SOURCE_DIR}/out)
cmake_path(SET PROTOBUF_DIR ${CMAKE_SOURCE_DIR}/protobuf)
cmake_path(SET TEST_DIR ${CMAKE_SOURCE_DIR}/test)

cmake_path(SET CMAKE_RUNTIME_OUTPUT_DIRECTORY ${BIN_DIR})
cmake_path(SET CMAKE_ARCHIVE_OUTPUT_DIRECTORY ${OUT_DIR})

file(MAKE_DIRECTORY ${CMD_BIN} ${TEST_BIN} ${TOOL_BIN} ${DEP_DIR} ${OUT_DIR} ${PROTOBUF_DIR})

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

	"github.com/containerd/containerd/cmd/protoc-gen-gogoctrd"
    "github.com/containerd/containerd/cmd/protoc-gen-go-fieldpath"
    "github.com/containerd/protobuild"
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
    set(env GOBIN=${go_bin})
    add_custom_command(OUTPUT "${${binary}_loc}"
        COMMAND ${CMAKE_COMMAND} -E env ${env} -- ${Go_EXECUTABLE} install ${i}
        COMMENT "installing ${binary} from ${i}"
        VERBATIM
    )
    add_custom_target(${binary} DEPENDS "${${binary}_loc}")
    set_target_properties(${binary} PROPERTIES FOLDER dependencies/go)
endforeach()

##########################################################################################
# build binaries
##########################################################################################

add_custom_target(build
    COMMENT "build all project go binaries"
)
set_target_properties(build PROPERTIES FOLDER binaries)

# go_build <package> <GOOS> [target_name]
function(go_build package os)
    cmake_path(GET package FILENAME binary)
    cmake_path(SET package ${CMAKE_SOURCE_DIR}/${package})
    cmake_path(SET out ${CMD_BIN}/${binary})
    set(flags -trimpath "-ldflags=-s -w")
    set(env CGO_ENABLED=0 GO111MODULE=on GOWORK=off GOOS=${os})

    if (os STREQUAL windows)
        cmake_path(APPEND_STRING out ".exe")
    endif()

    set(name ${binary})
    if ( ARGC GREATER 2 )
        set(name ${ARGV2})
    endif()

    add_custom_target(${name}
        BYPRODUCTS ${out}
        COMMAND ${CMAKE_COMMAND} -E env ${env} -- ${Go_EXECUTABLE} build ${flags} -o=${out}
        # COMMENT "building go binary ${binary} for ${os}"
        WORKING_DIRECTORY "${package}"
    )
    add_dependencies(build ${name})
    set_target_properties(${name} PROPERTIES FOLDER binaries)
    message(STATUS "go binary ${name} - ${out}")
endfunction()

go_build(cmd/containerd-shim-runhcs-v1 windows shim)
go_build(cmd/device-util windows)
go_build(cmd/gcs linux)
go_build(cmd/gcstools linux)
go_build(cmd/hooks/wait-paths linux)
go_build(cmd/ncproxy windows)
go_build(cmd/runhcs windows)
go_build(cmd/shimdiag windows)
go_build(cmd/tar2ext4 windows)
go_build(cmd/tar2ext4 linux tar2ext4-linux)
go_build(cmd/wclayer windows)
go_build(internal/tools/grantvmgroupaccess windows vmaccess)
go_build(internal/tools/uvmboot windows)
go_build(internal/tools/zapdir windows)


##########################################################################################
# go tests
##########################################################################################

add_custom_target(test
    COMMAND ${CMAKE_COMMAND} -E env GO111MODULE=on GOWORK=off -- ${gotestsum_loc} --format=testname -- -tags=admin "-gcflags=all=-d=checkptr" -v -timeout=30m -race ./...
    DEPENDS gotestsum
    COMMENT "running go tests"
    WORKING_DIRECTORY ${CMAKE_SOURCE_DIR}
    USES_TERMINAL
    VERBATIM
)
add_custom_target(test_hcn
    COMMAND ${CMAKE_COMMAND} -E env GO111MODULE=on GOWORK=off -- ${gotestsum_loc} --format=testname -- -tags=admin -tags=integration "-gcflags=all=-d=checkptr" -v -timeout=30m -race hcn/...
    DEPENDS gotestsum
    COMMENT "running go tests in ./hcn"
    WORKING_DIRECTORY ${CMAKE_SOURCE_DIR}
    USES_TERMINAL
    VERBATIM
)
add_custom_target(test_all DEPENDS test test_hcn)

##########################################################################################
# go mod tidy and vendor
##########################################################################################

add_custom_target(mod ALL)
set_target_properties(mod PROPERTIES FOLDER validation)

function(mod name cmd)
    set(env GO111MODULE=on GOWORK=off)
    set(flags "-e")

    add_custom_target(${name}
        COMMAND ${CMAKE_COMMAND} -E env ${env} -- ${Go_EXECUTABLE} mod ${cmd} ${ARGN}
        COMMENT "running go mod ${cmd} on ${CMAKE_CURRENT_SOURCE_DIR}"
        WORKING_DIRECTORY ${CMAKE_CURRENT_SOURCE_DIR}
        USES_TERMINAL
    )
    add_dependencies(mod ${name})
    set_target_properties(${name} PROPERTIES FOLDER validation)
    message(STATUS "go mod ${name}")
endfunction()

# order is important here (cmake alphabetizes dependencies, so declaration order not guaranteed)
mod(tidy-root tidy -v)
mod(vendor-root vendor)
add_dependencies(vendor-root tidy-root)

##########################################################################################
# go generate
##########################################################################################

add_custom_target(gen
    COMMAND ${CMAKE_COMMAND} -E env GO111MODULE=on GOWORK=off -- ${Go_EXECUTABLE} generate -x ./...
    DEPENDS mod
    COMMENT "running go generate"
    WORKING_DIRECTORY ${CMAKE_CURRENT_SOURCE_DIR}
)
set_target_properties(gen PROPERTIES FOLDER generation)

##########################################################################################
# lint
##########################################################################################

add_custom_target(lint)
set_target_properties(lint PROPERTIES FOLDER validation)

function(lint name os)
    set(env GO111MODULE=on GOWORK=off GOOS=${os})
    set(flags --timeout=2m --max-issues-per-linter=0 --max-same-issues=0 --modules-download-mode=readonly --verbose --config=${CMAKE_SOURCE_DIR}/.golangci.yml)

    add_custom_target(${name}
        COMMAND ${CMAKE_COMMAND} -E env ${env} -- ${Go_Linter_EXECUTABLE} run ${flags} ${ARGN}
        # COMMAND SET GOWORK=off&& SET GOOS=${os}&& ${Go_Linter_EXECUTABLE} run ${flags} ${ARGN}
        DEPENDS mod
        COMMENT "linting ${CMAKE_CURRENT_SOURCE_DIR} for ${os}"
        WORKING_DIRECTORY ${CMAKE_CURRENT_SOURCE_DIR}
        USES_TERMINAL
    )
    add_dependencies(lint ${name})
    set_target_properties(${name} PROPERTIES FOLDER validation)
    message(STATUS "golangci-lint ${name}")
endfunction()

lint(lint-root windows)
lint(lint-root-linux linux cmd/gcs/... cmd/gcstools/... internal/guest/... internal/tools/... ext4/... pkg/...)

##########################################################################################
# protobuf
##########################################################################################

add_custom_target(proto
    COMMAND ${CMAKE_COMMAND} -E env GO111MODULE=on GOWORK=off -- powershell.exe -Command "& \"${Go_EXECUTABLE}\" list ./... | Where-Object { \$_ -notlike \"*/vendor/*\" } | ForEach-Object { & \"${protobuild_loc}\" \$_ }"
    DEPENDS protobuild protoc-gen-gogoctrd
    COMMENT building protofiles
    WORKING_DIRECTORY ${CMAKE_SOURCE_DIR}
    VERBATIM
)
set_target_properties(proto PROPERTIES FOLDER generation)

##########################################################################################
# pr validation
##########################################################################################

add_custom_target(validate DEPENDS mod lint gen)
set_target_properties(validate PROPERTIES FOLDER validation)

##########################################################################################
# test
#
# add test subdirectory after all the custom commands and functions have been defined
##########################################################################################
add_subdirectory(test)
