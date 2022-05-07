package winapi

// for use with CreateRestrictedToken and other token functions
const (
	TOKEN_DISABLE_MAX_PRIVILEGE = 0x1
	TOKEN_SANDBOX_INERT         = 0x2
	TOKEN_LUA_TOKEN             = 0x4
	TOKEN_WRITE_RESTRICTED      = 0x8
)

// BOOL CreateRestrictedToken(
//   [in]           HANDLE               ExistingTokenHandle,
//   [in]           DWORD                Flags,
//   [in]           DWORD                DisableSidCount,
//   [in, optional] PSID_AND_ATTRIBUTES  SidsToDisable,
//   [in]           DWORD                DeletePrivilegeCount,
//   [in, optional] PLUID_AND_ATTRIBUTES PrivilegesToDelete,
//   [in]           DWORD                RestrictedSidCount,
//   [in, optional] PSID_AND_ATTRIBUTES  SidsToRestrict,
//   [out]          PHANDLE              NewTokenHandle
// );
//sys CreateRestrictedToken(existing windows.Token, flags uint32, disableSidCount uint32, sidsToDisable *byte, deletePrivilegeCount uint32, privilegesToDelete *byte, restrictedSidCount uint32, sidsToRestrict *byte, newToken *windows.Token) (err error) = advapi32.CreateRestrictedToken
