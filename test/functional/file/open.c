// Copyright 2017 Capsule8, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

#include <fcntl.h>
#include <stdio.h>
#include <unistd.h>

int main() {
	char pathname[4096];
	unsigned flags, mode;

	while (3 == scanf("%s %o %o", pathname, &flags, &mode)) {
		const int fd = open(pathname, flags, mode);

		if (fd >= 0) {
			printf("opened \"%s\", flags 0%08o, mode 0%08o, fd = %d\n", pathname, flags, mode, fd);
			close(fd);
		}
	}

	return 0;
}
