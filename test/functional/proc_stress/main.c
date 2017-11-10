#include <stdio.h>
#include <stdlib.h>

int main(int argc, char *argv[])
{
	const int retcode = (argc > 1) ? atoi(argv[1]) : 0;

	return retcode;
}
