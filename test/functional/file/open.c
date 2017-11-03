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
