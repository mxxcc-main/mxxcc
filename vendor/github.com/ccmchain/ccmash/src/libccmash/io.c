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
/** @file io.c
 * @author Lefteris Karapetsas <lefteris@ccmdev.com>
 * @date 2015
 */
#include "io.h"
#include <string.h>
#include <stdio.h>
#include <errno.h>

enum ccmash_io_rc ccmash_io_prepare(
	char const* dirname,
	ccmash_h256_t const seedhash,
	FILE** output_file,
	uint64_t file_size,
	bool force_create
)
{
	char mutable_name[DAG_MUTABLE_NAME_MAX_SIZE];
	enum ccmash_io_rc ret = CCMASH_IO_FAIL;
	// reset errno before io calls
	errno = 0;

	// assert directory exists
	if (!ccmash_mkdir(dirname)) {
		CCMASH_CRITICAL("Could not create the ccmash directory");
		goto end;
	}

	ccmash_io_mutable_name(CCMASH_REVISION, &seedhash, mutable_name);
	char* tmpfile = ccmash_io_create_filename(dirname, mutable_name, strlen(mutable_name));
	if (!tmpfile) {
		CCMASH_CRITICAL("Could not create the full DAG pathname");
		goto end;
	}

	FILE *f;
	if (!force_create) {
		// try to open the file
		f = ccmash_fopen(tmpfile, "rb+");
		if (f) {
			size_t found_size;
			if (!ccmash_file_size(f, &found_size)) {
				fclose(f);
				CCMASH_CRITICAL("Could not query size of DAG file: \"%s\"", tmpfile);
				goto free_memo;
			}
			if (file_size != found_size - CCMASH_DAG_MAGIC_NUM_SIZE) {
				fclose(f);
				ret = CCMASH_IO_MEMO_SIZE_MISMATCH;
				goto free_memo;
			}
			// compare the magic number, no need to care about endianess since it's local
			uint64_t magic_num;
			if (fread(&magic_num, CCMASH_DAG_MAGIC_NUM_SIZE, 1, f) != 1) {
				// I/O error
				fclose(f);
				CCMASH_CRITICAL("Could not read from DAG file: \"%s\"", tmpfile);
				ret = CCMASH_IO_MEMO_SIZE_MISMATCH;
				goto free_memo;
			}
			if (magic_num != CCMASH_DAG_MAGIC_NUM) {
				fclose(f);
				ret = CCMASH_IO_MEMO_SIZE_MISMATCH;
				goto free_memo;
			}
			ret = CCMASH_IO_MEMO_MATCH;
			goto set_file;
		}
	}
	
	// file does not exist, will need to be created
	f = ccmash_fopen(tmpfile, "wb+");
	if (!f) {
		CCMASH_CRITICAL("Could not create DAG file: \"%s\"", tmpfile);
		goto free_memo;
	}
	// make sure it's of the proper size
	if (fseek(f, (long int)(file_size + CCMASH_DAG_MAGIC_NUM_SIZE - 1), SEEK_SET) != 0) {
		fclose(f);
		CCMASH_CRITICAL("Could not seek to the end of DAG file: \"%s\". Insufficient space?", tmpfile);
		goto free_memo;
	}
	if (fputc('\n', f) == EOF) {
		fclose(f);
		CCMASH_CRITICAL("Could not write in the end of DAG file: \"%s\". Insufficient space?", tmpfile);
		goto free_memo;
	}
	if (fflush(f) != 0) {
		fclose(f);
		CCMASH_CRITICAL("Could not flush at end of DAG file: \"%s\". Insufficient space?", tmpfile);
		goto free_memo;
	}
	ret = CCMASH_IO_MEMO_MISMATCH;
	goto set_file;

	ret = CCMASH_IO_MEMO_MATCH;
set_file:
	*output_file = f;
free_memo:
	free(tmpfile);
end:
	return ret;
}
