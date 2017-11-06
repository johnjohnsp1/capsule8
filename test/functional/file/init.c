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
