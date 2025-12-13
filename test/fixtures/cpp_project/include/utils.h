#ifndef UTILS_H
#define UTILS_H

#include <string>
#include <vector>

#define VERSION_MAJOR 1
#define VERSION_MINOR 0
#define DEBUG_MODE 1

namespace utils {
    std::string trim(const std::string& str);
    std::vector<std::string> split(const std::string& str, char delimiter);
    bool fileExists(const std::string& path);
}

#endif // UTILS_H
