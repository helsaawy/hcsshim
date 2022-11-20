# use vswhere to call the developer's prompt when using any generator besides Visual Studio

message(CHECK_START "finding vswhere.exe")

# typically at ${env:ProgramFiles(x86)}\Microsoft Visual Studio\Installer
find_program(VSWHERE vswhere PATH_SUFFIXES Microsoft Visual Studio/Installer)
if(EXISTS ${VSWHERE})
    message(CHECK_PASS ${VSWHERE})

    # check cache for dev command prompt first
    message(CHECK_START "finding vsdevcmd.bat")
    if(NOT EXISTS ${VS_DEV_BAT})
        execute_process(
            COMMAND ${VSWHERE} -prerelease -latest -property installationPath
            OUTPUT_VARIABLE VS_INSTALL
            RESULT_VARIABLE CODE
        )
        string(STRIP ${VS_INSTALL} VS_INSTALL)
        if( (NOT "${CODE}" STREQUAL "0") OR (NOT EXISTS ${VS_INSTALL}) )
            message(CHECK_FAIL "Could not get Visual Studio tools installation path: ${VS_INSTALL} - ${CODE}")
        else()
            # if its not found here, something's fucky
            find_program(VS_DEV_BAT vsdevcmd.bat PATH ${VS_INSTALL}/Common7/Tools REQUIRED)
            message(CHECK_PASS ${VS_DEV_BAT})
        endif()

        set(VS_CMD $ENV{COMSPEC} /c ${VS_DEV_BAT} -no_logo &&)

        unset(VS_INSTALL)
        unset(CODE)
    else()
        message(CHECK_PASS ${VS_DEV_BAT})
    endif()
else()
    message(CHECK_FAIL "Visual Studio tools not installed")
    unset(VSWHERE CACHE)
endif()