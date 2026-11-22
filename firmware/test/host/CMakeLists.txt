cmake_minimum_required(VERSION 3.16)
project(garden_core_host_tests C)
set(CMAKE_C_STANDARD 11)
add_compile_options(-Wall -Wextra -Werror)

include(FetchContent)
FetchContent_Declare(unity
    GIT_REPOSITORY https://github.com/ThrowTheSwitch/Unity.git
    GIT_TAG v2.6.0)
FetchContent_MakeAvailable(unity)

set(CORE ${CMAKE_CURRENT_LIST_DIR}/../../components/garden_core)
add_library(garden_core STATIC
    ${CORE}/telemetry_payload.c ${CORE}/offline_buffer.c ${CORE}/median.c)
target_include_directories(garden_core PUBLIC ${CORE}/include)
target_link_libraries(garden_core PUBLIC m)

enable_testing()
foreach(t test_payload test_offline_buffer test_median)
    add_executable(${t} ${t}.c)
    target_link_libraries(${t} PRIVATE garden_core unity)
    target_compile_definitions(${t} PRIVATE GOLDEN_DIR="${CMAKE_CURRENT_LIST_DIR}/../golden/v1")
    add_test(NAME ${t} COMMAND ${t})
endforeach()
