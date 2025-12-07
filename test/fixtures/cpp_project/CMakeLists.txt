cmake_minimum_required(VERSION 3.14)
project(MyApp VERSION 1.0.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

find_package(Threads REQUIRED)

add_library(utils STATIC
    src/utils.cpp
)
target_include_directories(utils PUBLIC include)

add_executable(myapp
    src/main.cpp
    src/platform_win.cpp
    src/platform_linux.cpp
)
target_link_libraries(myapp PRIVATE utils Threads::Threads)

enable_testing()
add_executable(test_main tests/test_main.cpp)
target_link_libraries(test_main PRIVATE utils)
add_test(NAME unit_tests COMMAND test_main)
