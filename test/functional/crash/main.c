#include <unistd.h>

int main() {
  sleep(1);
  int *ptr = (int*)0x88888888;
  (*ptr)++;
  return 0;
}
