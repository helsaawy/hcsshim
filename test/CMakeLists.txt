cmake_minimum_required(VERSION 3.25)

project(hcsshim-tests
    DESCRIPTION "hcsshim testing binaries"
    LANGUAGES NONE
)
##########################################################################################
# build go test binaries
##########################################################################################

add_custom_target(build_test
    COMMENT "build testing binaries"
)

# go_build_test <package> <GOOS> <use race detector> [target_name]
function(go_build_test package os race)
    cmake_path(GET package FILENAME binary)
    cmake_path(APPEND_STRING binary ".test")
    cmake_path(SET package ${CMAKE_CURRENT_SOURCE_DIR}/${package})
    cmake_path(SET out ${TEST_BIN}/${binary})
    set(env GO111MODULE=on GOWORK=off GOOS=${os})
    set(flags "-tags=functional" "-gcflags=all=-d=checkptr")

    if (os STREQUAL windows)
        cmake_path(APPEND_STRING out ".exe")
    endif()

    set(cgo 0)
    if (race)
        set(cgo 1)
        string(APPEND flags " -race")
    endif()
    list(APPEND env "CGO_ENABLED=${cgo}")

    if ( ARGC GREATER 3 )
        set(name ${ARGV3})
    else()
        set(name ${binary})
    endif()

    add_custom_target(${name}
        BYPRODUCTS ${out}
        COMMAND ${CMAKE_COMMAND} -E env ${env} -- ${Go_EXECUTABLE} test ${flags} -c -o=${out}
        COMMENT "building go test binary ${binary} for ${os}"
        WORKING_DIRECTORY "${package}"
    )
    add_dependencies(build_test ${name})
    message(STATUS "go test binary ${name} - ${out}")
endfunction()

go_build_test(containerd-shim-runhcs-v1 windows false shim.test)
go_build_test(cri-containerd windows false cri.test)
go_build_test(functional windows true func.test)
go_build_test(runhcs windows false)
go_build_test(gcs linux false)

##########################################################################################
# go mod tidy and vendor
##########################################################################################

mod(tidy-test tidy -v)
add_dependencies(tidy-test tidy-root)

##########################################################################################
# lint
##########################################################################################

lint(lint-test windows)
lint(lint-test-linux linux)
