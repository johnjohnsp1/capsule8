#include <stdlib.h>
#include <sys/types.h>
#include <signal.h>
#include <stdlib.h>

int main(int argc, char* argv[]) {
	kill(0, atoi(argv[1]));
}
