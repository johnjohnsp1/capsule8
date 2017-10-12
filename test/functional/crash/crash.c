// Simple segfault test.
// Compile with cc -o segfault -static segfault.c
int main() {
  int *ptr = 0x88888888;
  (*ptr)++;
  return 0;
}
