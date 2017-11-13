#include <unistd.h>

#ifndef ALARM_SECS
#define ALARM_SECS 37
#endif

int main(int argc, char *argv[]) {
	alarm(ALARM_SECS);
	alarm(0);
	return 0;
}
