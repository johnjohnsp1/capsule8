// Copyright 2017 Capsule8, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package functional

import (
	"fmt"
	"io"
	"os"
	"syscall"

	api "github.com/capsule8/capsule8/api/v0"
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
