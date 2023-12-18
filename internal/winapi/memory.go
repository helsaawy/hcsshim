package winapi

//sys LocalAlloc(flags uint32, size int) (ptr uintptr) = kernel32.LocalAlloc
//sys LocalFree(ptr uintptr) = kernel32.LocalFree

//	BOOL GetPhysicallyInstalledSystemMemory(
//	 [out] PULONGLONG TotalMemoryInKilobytes
//	);
//sys getPhysicallyInstalledSystemMemory(TotalMemoryInKilobytes *uint64) (err error)= kernel32.GetPhysicallyInstalledSystemMemory

func GetPhysicallyInstalledSystemMemory() (kb uint64, _ error) {
	return kb, getPhysicallyInstalledSystemMemory(&kb)
}
