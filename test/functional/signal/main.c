#include <stdlib.h>
#include <sys/types.h>
#include <signal.h>
#include <stdlib.h>
#include <unistd.h>

int main(int argc, char* argv[]) {
	raise(SIGUSR1);

	// This should not be returned
	return -1;
}
