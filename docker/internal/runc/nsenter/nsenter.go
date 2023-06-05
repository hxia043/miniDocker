package nsenter

/*
#include <errno.h>
#include <sched.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <fcntl.h>

__attribute__((constructor)) void enter_namespace(void) {
	char *minidocker_pid;
	minidocker_pid = getenv("minidocker_pid");
	if (minidocker_pid) {
		//fprintf(stdout, "got minidocker_pid=%s\n", minidocker_pid);
	} else {
		//fprintf(stdout, "missing minidocker_pid env skip nsenter");
		return;
	}
	char *minidocker_cmd;
	minidocker_cmd = getenv("minidocker_cmd");
	if (minidocker_cmd) {
		//fprintf(stdout, "got minidocker_cmd=%s\n", minidocker_cmd);
	} else {
		//fprintf(stdout, "missing minidocker_cmd env skip nsenter");
		return;
	}
	int i;
	char nspath[1024];
	char *namespaces[] = { "ipc", "uts", "net", "pid", "mnt" };

	for (i=0; i<5; i++) {
		sprintf(nspath, "/proc/%s/ns/%s", minidocker_pid, namespaces[i]);
		int fd = open(nspath, O_RDONLY);

		if (setns(fd, 0) == -1) {
			//fprintf(stderr, "setns on %s namespace failed: %s\n", namespaces[i], strerror(errno));
		} else {
			//fprintf(stdout, "setns on %s namespace succeeded\n", namespaces[i]);
		}
		close(fd);
	}
	int res = system(minidocker_cmd);
	exit(0);
	return;
}
*/
import "C"
