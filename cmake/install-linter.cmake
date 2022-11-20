# install golangci-lint from github

# check the cached version first
# if(NOT EXISTS ${Go_Linter_EXECUTABLE})
#     set(lint_version "1.50.0")
#     cmake_path(SET lint_archive ${OUT_DIR}/golangci-lint${archive_ext})
#     set(curl_flags -L --no-progress-meter --write-out "code: %{http_code}\ncontent: %{content_type}\nbytes: %{size_download}\nurl: %{url_effective}\n" --tls-max "1.2")
#     set(lint_dir golangci-lint-${lint_version}-${os}-${arch})
#     set(lint_url "https://github.com/golangci/golangci-lint/releases/download/v${lint_version}/${lint_dir}${archive_ext}")

#     message(STATUS "downloading golangci-lint v${lint_version}")
#     execute_process(
#         COMMAND curl ${curl_flags} --output ${lint_archive} ${lint_url}
#         RESULT_VARIABLE res
#     )

#     # can't use file(DOWNLOAD) because windows 11 bumps up to TLS1.3, but that is still unsupported and errors out
#     # file(DOWNLOAD
#     #     ${lint_url}
#     #     ${lint_archive}
#     #     HTTPHEADER "Accept: application/octet-stream"
#     #     SHOW_PROGRESS
#     #     STATUS status
#     #     LOG log
#     # )

#     if (res)
#         message(WARNING "could not download golangci-lint ${res}")
#         set(res 0)
#     else()
#         file(ARCHIVE_EXTRACT
#             INPUT ${lint_archive}
#             DESTINATION ${TOOL_BIN}
#             PATTERNS "**/golangci-lint*"
#         )
#     endif()

#     if (res)
#         message(WARNING "could not unpack golangci-lint ${res}")
#     else()
#     endif()

#     unset(curl_flags)
#     unset(lint_url)
#     unset(lint_dir)
#     unset(lint_archive)
#     unset(lint_version)
# endif()