#include <netdb.h>
#include <netinet/in.h>
#include <netinet/ip.h> /* superset of previous */
#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/types.h>
#include <sys/socket.h>
#include <unistd.h>

#define handle_error(msg) \
	do { perror(msg); exit(EXIT_FAILURE); } while (0)

static void* client_routine(void* data) {
	const struct sockaddr_in *addr = data;
	static const char *msg = "Hello, World!\n";

	int cfd = socket(AF_INET, SOCK_STREAM, 0);
	if (cfd == -1) {
		handle_error("socket");
	}

	if (connect(cfd, (const struct sockaddr*) addr, sizeof(*addr)) < 0) {
		handle_error("connect");
	}

	if (sendto(cfd, msg, strlen(msg), 0, NULL, 0) < 0) {
		handle_error("sendto");
	}

	close(cfd);
	return NULL;
}

int main(int argc, char *argv[]) {
	int sfd;
	pthread_t client;
	unsigned short port;

	if (argc < 2 || sscanf(argv[1], "%hu", &port) < 1) {
		port = 80;
	}


	sfd = socket(AF_INET, SOCK_STREAM, 0);
	if (sfd == -1) {
		handle_error("socket");
	}

	struct sockaddr_in addr;
	memset(&addr, 0, sizeof(addr));
	addr.sin_family = AF_INET;
	addr.sin_port = htons(port);
	struct hostent* lh = gethostbyname("localhost");
	if (lh == NULL) {
		handle_error("gethostbyname");
	}
	addr.sin_addr = *(struct in_addr *) lh->h_addr_list[0];

	if (bind(sfd, (struct sockaddr *) &addr, sizeof(addr)) == -1) {
		handle_error("bind");
	}

	if (listen(sfd, 1) == -1) {
		handle_error("listen");
	}

	pthread_create(&client, NULL, client_routine, &addr);

	int afd = accept(sfd, NULL, NULL);
	if (afd < 0) {
		handle_error("accept");
	}

	char buffer[80];
	ssize_t len = recvfrom(afd, buffer, sizeof(buffer), 0, NULL, 0);
	if (len < 0) {
		handle_error("recvfrom");
	}
	buffer[len] = 0;
	fputs(buffer, stdout);

	pthread_join(client, NULL);

	close(sfd);
	return 0;
}
