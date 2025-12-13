#include "utils.h"
#include <iostream>
#include <cstdlib>

#ifdef _WIN32
    #include <windows.h>
    #define PLATFORM "Windows"
#elif defined(__linux__)
    #include <unistd.h>
    #define PLATFORM "Linux"
#elif defined(__APPLE__)
    #include <TargetConditionals.h>
    #define PLATFORM "macOS"
#else
    #define PLATFORM "Unknown"
#endif

#ifndef CONFIG_FILE
    #define CONFIG_FILE "/etc/myapp/config.ini"
#endif

extern void initPlatform();
extern void cleanupPlatform();

int main(int argc, char* argv[]) {
    std::cout << "Platform: " << PLATFORM << std::endl;
    std::cout << "Version: " << VERSION_MAJOR << "." << VERSION_MINOR << std::endl;

    initPlatform();

    if (argc > 1) {
        std::string arg = utils::trim(argv[1]);
        std::cout << "Argument: " << arg << std::endl;
    }

    cleanupPlatform();
    return EXIT_SUCCESS;
}
