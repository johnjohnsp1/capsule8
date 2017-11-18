#include <unistd.h>

int main() {
  int *ptr = (int*)0x88888888;
  (*ptr)++;
  return 0;
}
