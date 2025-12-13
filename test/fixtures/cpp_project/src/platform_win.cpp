#ifdef _WIN32

#include <windows.h>
#include <iostream>

#define WIN_FEATURE_ENABLED 1

void initPlatform() {
    #ifdef WIN_FEATURE_ENABLED
    std::cout << "Initializing Windows platform..." << std::endl;
    // Windows-specific initialization
    SetConsoleOutputCP(CP_UTF8);
    #endif
}

void cleanupPlatform() {
    std::cout << "Cleaning up Windows resources..." << std::endl;
}

#else
// Empty stubs for non-Windows platforms
void initPlatform() {}
void cleanupPlatform() {}
#endif
