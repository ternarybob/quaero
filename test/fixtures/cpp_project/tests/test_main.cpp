// Test file using Google Test pattern
#include "utils.h"
#include <cassert>
#include <iostream>

// Simulated gtest-like macros for testing
#define TEST(suite, name) void test_##suite##_##name()
#define EXPECT_EQ(a, b) assert((a) == (b))
#define EXPECT_TRUE(x) assert(x)
#define EXPECT_FALSE(x) assert(!(x))

TEST(Utils, Trim) {
    EXPECT_EQ(utils::trim("  hello  "), "hello");
    EXPECT_EQ(utils::trim(""), "");
    EXPECT_EQ(utils::trim("no_spaces"), "no_spaces");
}

TEST(Utils, Split) {
    auto result = utils::split("a,b,c", ',');
    EXPECT_EQ(result.size(), 3);
    EXPECT_EQ(result[0], "a");
    EXPECT_EQ(result[1], "b");
    EXPECT_EQ(result[2], "c");
}

TEST(Utils, FileExists) {
    EXPECT_FALSE(utils::fileExists("/nonexistent/path/file.txt"));
}

int main() {
    std::cout << "Running unit tests..." << std::endl;

    test_Utils_Trim();
    std::cout << "  [PASS] Utils.Trim" << std::endl;

    test_Utils_Split();
    std::cout << "  [PASS] Utils.Split" << std::endl;

    test_Utils_FileExists();
    std::cout << "  [PASS] Utils.FileExists" << std::endl;

    std::cout << "All tests passed!" << std::endl;
    return 0;
}
