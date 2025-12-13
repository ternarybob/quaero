#ifdef __linux__

#include <unistd.h>
#include <sys/types.h>
#include <iostream>

#define LINUX_FEATURE_ENABLED 1

void initPlatform() {
    #ifdef LINUX_FEATURE_ENABLED
    std::cout << "Initializing Linux platform..." << std::endl;
    // Linux-specific initialization
    #endif
}

void cleanupPlatform() {
    std::cout << "Cleaning up Linux resources..." << std::endl;
}

#else
// Empty stubs for non-Linux platforms
void initPlatform() {}
void cleanupPlatform() {}
#endif
