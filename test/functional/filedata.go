package functional

import (
	"fmt"
	"io"
	"os"
	"syscall"

	api "github.com/capsule8/api/v0"
)

func fileTestDataMap() (map[string]*api.FileEvent, error) {
	file, err := os.Open("file/testdata/filedata.txt")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	result := make(map[string]*api.FileEvent)
	for {
		var path string
		var flags, mode uint
		_, err = fmt.Fscanf(file, "%s %o %o\n", &path, &flags, &mode)
		if err == io.EOF {
			break
		} else if err != nil {
			return result, err
		}

		result[path] = &api.FileEvent{
			Type:      api.FileEventType_FILE_EVENT_TYPE_OPEN,
			Filename:  path,
			OpenFlags: int32(flags),
			OpenMode:  int32(mode),
		}
	}

	return result, nil
}

func eventMatchFileTestData(fe *api.FileEvent, td *api.FileEvent) bool {
	// Even on 64 bit systems, we see the large file bit set in open flags.
	// For now, ignore differences in this bit.
	const O_LARGEFILE = 0100000
	return fe.Type == td.Type &&
		fe.Filename == td.Filename &&
		(fe.OpenFlags & ^O_LARGEFILE) == (td.OpenFlags & ^O_LARGEFILE) &&
		(fe.OpenMode == td.OpenMode || (fe.OpenFlags&syscall.O_CREAT) == 0)
}
