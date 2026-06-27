#pragma once
#include <cstdint>
#include <cstddef>

bool ota_in_progress();
void ota_abort();
bool ota_begin(size_t size, const char* md5, size_t chunkSize);
bool ota_write_chunk(const uint8_t* data, size_t len);
bool ota_commit();
