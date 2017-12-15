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
#include <sys/stat.h>
#include <unistd.h>

int main() {
	char pathname[4096];
	unsigned flags, mode;

	while (3 == scanf("%s %o %o", pathname, &flags, &mode)) {
		if ((flags & O_CREAT) && !(mode & S_IFCHR)) {
			continue;
		} else if (mode & S_IFDIR) {
			mkdir(pathname, mode);
		} else {
			const int fd = open(pathname, flags | O_CREAT, mode);
			if (fd >= 0) {
				close(fd);
			}
		}
	}

	return 0;
}
