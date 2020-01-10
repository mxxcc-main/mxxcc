/*
  This file is part of ccmash.

  ccmash is free software: you can redistribute it and/or modify
  it under the terms of the GNU General Public License as published by
  the Free Software Foundation, either version 3 of the License, or
  (at your option) any later version.

  ccmash is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  along with ccmash.  If not, see <http://www.gnu.org/licenses/>.
*/

/** @file ccmash.h
* @date 2015
*/
#pragma once

#include <stdint.h>
#include <stdbool.h>
#include <string.h>
#include <stddef.h>
#include "compiler.h"

#define CCMASH_REVISION 23
#define CCMASH_DATASET_BYTES_INIT 1073741824U // 2**30
#define CCMASH_DATASET_BYTES_GROWTH 8388608U  // 2**23
#define CCMASH_CACHE_BYTES_INIT 1073741824U // 2**24
#define CCMASH_CACHE_BYTES_GROWTH 131072U  // 2**17
#define CCMASH_EPOCH_LENGTH 30000U
#define CCMASH_MIX_BYTES 128
#define CCMASH_HASH_BYTES 64
#define CCMASH_DATASET_PARENTS 256
#define CCMASH_CACHE_ROUNDS 3
#define CCMASH_ACCESSES 64
#define CCMASH_DAG_MAGIC_NUM_SIZE 8
#define CCMASH_DAG_MAGIC_NUM 0xFEE1DEADBADDCAFE

#ifdef __cplusplus
extern "C" {
#endif

/// Type of a seedhash/blockhash e.t.c.
typedef struct ccmash_h256 { uint8_t b[32]; } ccmash_h256_t;

// convenience macro to statically initialize an h256_t
// usage:
// ccmash_h256_t a = ccmash_h256_static_init(1, 2, 3, ... )
// have to provide all 32 values. If you don't provide all the rest
// will simply be unitialized (not guranteed to be 0)
#define ccmash_h256_static_init(...)			\
	{ {__VA_ARGS__} }

struct ccmash_light;
typedef struct ccmash_light* ccmash_light_t;
struct ccmash_full;
typedef struct ccmash_full* ccmash_full_t;
typedef int(*ccmash_callback_t)(unsigned);

typedef struct ccmash_return_value {
	ccmash_h256_t result;
	ccmash_h256_t mix_hash;
	bool success;
} ccmash_return_value_t;

/**
 * Allocate and initialize a new ccmash_light handler
 *
 * @param block_number   The block number for which to create the handler
 * @return               Newly allocated ccmash_light handler or NULL in case of
 *                       ERRNOMEM or invalid parameters used for @ref ccmash_compute_cache_nodes()
 */
ccmash_light_t ccmash_light_new(uint64_t block_number);
/**
 * Frees a previously allocated ccmash_light handler
 * @param light        The light handler to free
 */
void ccmash_light_delete(ccmash_light_t light);
/**
 * Calculate the light client data
 *
 * @param light          The light client handler
 * @param header_hash    The header hash to pack into the mix
 * @param nonce          The nonce to pack into the mix
 * @return               an object of ccmash_return_value_t holding the return values
 */
ccmash_return_value_t ccmash_light_compute(
	ccmash_light_t light,
	ccmash_h256_t const header_hash,
	uint64_t nonce
);

/**
 * Allocate and initialize a new ccmash_full handler
 *
 * @param light         The light handler containing the cache.
 * @param callback      A callback function with signature of @ref ccmash_callback_t
 *                      It accepts an unsigned with which a progress of DAG calculation
 *                      can be displayed. If all goes well the callback should return 0.
 *                      If a non-zero value is returned then DAG generation will stop.
 *                      Be advised. A progress value of 100 means that DAG creation is
 *                      almost complete and that this function will soon return succesfully.
 *                      It does not mean that the function has already had a succesfull return.
 * @return              Newly allocated ccmash_full handler or NULL in case of
 *                      ERRNOMEM or invalid parameters used for @ref ccmash_compute_full_data()
 */
ccmash_full_t ccmash_full_new(ccmash_light_t light, ccmash_callback_t callback);

/**
 * Frees a previously allocated ccmash_full handler
 * @param full    The light handler to free
 */
void ccmash_full_delete(ccmash_full_t full);
/**
 * Calculate the full client data
 *
 * @param full           The full client handler
 * @param header_hash    The header hash to pack into the mix
 * @param nonce          The nonce to pack into the mix
 * @return               An object of ccmash_return_value to hold the return value
 */
ccmash_return_value_t ccmash_full_compute(
	ccmash_full_t full,
	ccmash_h256_t const header_hash,
	uint64_t nonce
);
/**
 * Get a pointer to the full DAG data
 */
void const* ccmash_full_dag(ccmash_full_t full);
/**
 * Get the size of the DAG data
 */
uint64_t ccmash_full_dag_size(ccmash_full_t full);

/**
 * Calculate the seedhash for a given block number
 */
ccmash_h256_t ccmash_get_seedhash(uint64_t block_number);

#ifdef __cplusplus
}
#endif
