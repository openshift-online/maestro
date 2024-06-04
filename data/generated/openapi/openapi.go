// Code generated for package openapi by go-bindata DO NOT EDIT. (@generated)
// sources:
// ../../openapi/openapi.yaml
package openapi

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func bindataRead(data []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("Read %q: %v", name, err)
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, gz)
	clErr := gz.Close()

	if err != nil {
		return nil, fmt.Errorf("Read %q: %v", name, err)
	}
	if clErr != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

type asset struct {
	bytes []byte
	info  os.FileInfo
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

// Name return file name
func (fi bindataFileInfo) Name() string {
	return fi.name
}

// Size return file size
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}

// Mode return file mode
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}

// Mode return file modify time
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}

// IsDir return file whether a directory
func (fi bindataFileInfo) IsDir() bool {
	return fi.mode&os.ModeDir != 0
}

// Sys return file is sys mode
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var _openapiYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xec\x5c\x5f\x8f\xdb\xb8\x11\x7f\xdf\x4f\x31\x40\x5b\xf8\x72\xf0\xda\x4e\xef\x0a\xb4\x46\x72\x40\x72\xbd\x14\x77\xc8\x25\x69\x36\x69\x1f\x8a\xc2\x4b\x93\x23\x8b\x89\x44\x2a\x24\xb5\x59\xa7\xed\x77\x2f\x48\xea\xbf\x25\xad\xec\xee\xc6\xda\xad\xf3\x92\x35\x35\x1c\xce\x90\xbf\xf9\x69\x48\x8e\x2d\x13\x14\x24\xe1\x4b\xf8\x6e\xb6\x98\x2d\xce\xb8\x08\xe4\xf2\x0c\xc0\x70\x13\xe1\x12\x62\x82\xda\x28\x09\x17\xa8\xae\x38\x45\x78\xf6\xe6\xe7\x33\x00\x86\x9a\x2a\x9e\x18\x2e\x45\x97\xc8\x15\x2a\xed\x1e\x2f\x66\x8b\xd9\xe3\x33\x8d\xca\xb6\x58\xcd\xe7\x90\xaa\x68\x09\xa1\x31\xc9\x72\x3e\x8f\x24\x25\x51\x28\xb5\x59\xfe\x71\xb1\x58\x9c\x01\x34\xb4\xd3\x54\x29\x14\x06\x98\x8c\x09\x17\xf5\xee\x7a\x39\x9f\x93\x84\xcf\xac\x0b\x3a\xe4\x81\x99\x51\x19\xef\xaa\xf8\x95\x70\x01\xdf\x24\x4a\xb2\x94\xda\x96\x47\xe0\xad\x69\x57\xa6\x0d\xd9\xe0\x4d\x2a\x2f\x0c\xd9\x70\xb1\xc9\x15\x25\xc4\x84\xce\x37\xab\x61\x9e\x4d\xc8\xfc\xea\xf1\x5c\xa1\x96\xa9\xa2\xe8\x1e\x02\x6c\xd0\xf8\x3f\x00\x74\x1a\xc7\x44\x6d\x97\xf0\x16\x4d\xaa\x84\x06\x02\x11\xd7\x06\x64\x00\x45\xa7\x5c\x14\x69\xaa\xb8\xd9\xe6\x5d\xad\xd9\xcf\x91\x28\x54\x4b\xf8\xc7\x3f\xb3\x46\x85\x3a\x91\x42\xe7\x23\xd9\x7f\x93\xdf\x2f\x16\x93\xf2\x63\xc3\x85\x67\xf0\xcb\xc5\xeb\x57\x40\x94\x22\xdb\xea\xa8\x20\xd7\x1f\x90\x1a\x5d\xe9\x47\xa5\x30\x28\x4c\x55\x15\x00\x49\x92\x88\x53\x62\x95\xcd\x3f\x68\x29\xea\x4f\x01\x34\x0d\x31\x26\xcd\x56\x80\xdf\x2a\x0c\x96\x30\xf9\xcd\x9c\xca\x38\x91\x02\x85\xd1\x73\x2f\xab\xe7\x6f\x33\x1b\x5e\x72\x6d\x26\xa5\x1f\xdf\x2f\x1e\xf7\xf8\x91\x9a\x10\x8c\xfc\x88\x02\xb8\x06\x2e\xae\x48\xc4\xd9\x31\x8c\xff\x49\x29\xa9\x6a\x56\x7f\xd7\x6d\xf5\x7b\x41\x52\x13\x4a\xc5\xbf\x20\x03\x23\x21\x41\x15\x48\x15\x83\x4c\x50\x39\xb3\xc6\xe0\xc1\x1f\xfa\xf0\xf3\x5e\xe0\x75\x82\xd4\x20\x03\xb4\xfd\x40\x52\x17\xab\xc7\x9f\xfb\x84\x28\x12\xa3\xc9\xe8\x06\x5c\xbc\xb4\x75\x2e\xe5\xe6\x09\xd9\xe0\x64\xa8\xb0\xe6\x5f\xf6\x10\x46\xa2\x68\x38\x58\x5c\x2a\x86\xea\xf9\x76\xb0\x7c\xc0\x31\x62\xda\x8b\x27\x96\x45\x9b\xf4\xf2\xa3\x42\x62\x10\x08\x08\xfc\x5c\xc4\xf8\x7e\xc4\xf2\x29\x45\x6d\x9e\x4b\x56\x91\xab\x21\x21\x8f\x5a\x60\xc4\x90\x42\xc4\xf6\xe3\x0a\xd9\x12\x8c\x4a\xf1\xac\x07\x12\xfd\x80\x68\x87\xc3\x10\x16\x99\xf4\x52\x63\x0f\xa5\xf8\x39\x3b\x0a\x90\x9b\xb6\x3b\x1e\xe9\x89\xc2\xbf\x59\xb6\x73\x26\xf8\x28\xd4\xe3\x09\xc3\x13\x71\x1f\xd1\x83\x3f\x75\x7b\x50\x84\x2b\x89\x14\x12\xb6\x05\xbc\xe6\xfa\x38\xef\xfb\xbd\x5e\x38\xcf\x04\xa4\x5d\xef\x1c\xa0\x36\x64\x6d\x46\x66\x42\x6c\xd2\xdc\x71\x5c\xea\x4c\x05\xe7\xff\xe2\xec\x3f\xdd\xf9\xe0\x5f\xd0\x00\x11\x65\x3a\xb6\xde\x42\x11\x16\x77\x93\x09\x16\x80\x08\x64\x2a\x58\x6d\xc0\xaf\x3a\x75\xad\xdc\x77\x22\x90\xe3\x78\xf0\x7d\xb7\x07\xaf\x64\x89\xce\xcf\xdc\x84\xa0\x13\xa4\x3c\xe0\xc8\x80\xb3\xfb\xc2\x26\x63\x4d\x5f\x13\x62\x68\xb8\x43\x0a\xef\x13\xe6\xb2\x38\x71\x47\x29\x9c\xd7\xcf\xca\x75\x1d\x59\x2a\xf7\xc6\xce\xca\x5b\xef\x46\x7f\x5a\x37\x84\xe7\xd2\xcc\x5b\x9d\x52\x8a\x5a\x07\x69\x14\x6d\x47\x43\x78\xa7\x64\xef\x2b\x5b\x7d\xe2\xea\x51\x38\xf1\x00\x33\xd6\x9d\x77\x8c\x23\x1e\x9b\xa5\x8e\x22\x43\xb5\xd6\x46\x68\x70\xe7\x6d\xf3\x67\xd7\x0c\xe4\xc0\x97\x4d\x1b\x2d\xf7\x40\xb4\x3c\x3e\x70\xc3\x76\xd0\xf2\x89\x19\xbf\xbe\xd5\x27\x66\x1c\x81\x13\xfb\x31\x8c\x8b\xa1\x11\x31\x4c\xf3\x2c\xf6\xc6\x03\x4d\xce\xfa\x36\xcf\xe7\xeb\x54\xb0\xe8\xb0\xeb\x14\xc8\xfa\xde\xe9\x5e\xba\xf3\x56\xc5\x0f\x3e\x86\xcb\x95\xe7\xce\x92\xd3\x15\xcb\x28\x28\xea\x5e\xee\x51\xff\x5f\xaf\x58\x6e\x62\xa5\x7d\x4f\xf6\x3c\x25\x7c\xc5\x03\xbe\x6c\xc4\x91\x9c\xf3\x79\x22\x3a\x91\xd0\x08\x3c\x18\x98\x27\x65\xf8\x79\x38\xe9\xd2\x7d\x26\xd4\xf6\x4c\x89\x4a\xa1\xd3\xb8\xd0\x33\x2c\x45\x2a\x3a\x7d\xd5\xdc\x28\x1f\xf5\x98\x49\xd1\x8f\x99\x0d\xa7\x74\x68\x14\x4c\xf4\x60\xa2\x77\xcf\x84\x68\xcf\x94\x68\xef\xa4\x68\xff\xb4\xe8\xd6\x6b\x4f\xf2\x68\xdf\x8f\x62\x6e\xba\xb8\xc8\xe3\x77\x2c\x17\x16\xb9\x3d\xf7\xb1\xf6\xa4\x69\xfb\xe9\xd0\xed\x44\xe1\x87\x78\xd0\x73\x92\x5f\x84\xeb\x3d\x3b\xc9\x1f\x5e\x7b\xd2\xa0\xb9\xe3\xb8\xd4\x99\x14\x0e\xdb\xa1\x16\x89\xd9\xdd\x6f\x4d\x0b\x40\x1c\x79\x4f\xda\xca\x7d\x27\x02\x19\xe3\x6e\xb4\x40\xe7\x69\x1b\x7a\xeb\x87\xf5\xfd\xb5\x27\x77\x93\xc2\xe5\xb5\x27\x74\xa4\xa9\xdc\xad\xd4\x9e\x14\x3c\x37\x96\xda\x93\x53\xb2\x37\x06\xab\x4f\x5c\x3d\x0a\x27\x1e\x60\xc6\xda\x5d\x7b\x32\x8a\x0c\xf5\xe6\xda\x93\xc3\x5e\x36\x7b\xd6\x9e\x94\xc7\x07\xa7\xda\x93\x13\x33\xde\xae\x07\x0f\x80\x19\x0f\xac\x3d\x19\x09\xc3\x1c\x78\xa7\x52\x3e\xb1\xdd\x72\xde\xb9\xb0\xfa\x73\x62\xc9\x88\x27\xd3\x6a\xb6\x09\xfa\xef\x10\x9f\x55\xec\xc6\x25\xac\x9d\x58\xd6\xe8\x3f\xbc\x90\x2a\x26\x66\x09\xbf\xfc\xfd\xdd\x59\xee\x60\xa6\xf4\xb5\xbb\x05\x79\x8b\x01\x2a\x14\x14\xeb\xda\xfd\x15\x49\xd6\x94\x28\x0b\x75\xc3\xab\x3c\xc7\x59\x75\x9e\x7c\x27\x6d\x14\x17\x9b\xa2\xf9\x23\x17\x37\x0b\x85\x76\x82\xfa\x84\x5e\xf2\xf2\xa4\x77\xa0\x6d\x83\x06\x4e\xc8\x06\x77\x85\xb8\x30\xb8\xa9\x20\x49\xf3\x2f\x03\xa4\x8c\x34\x24\xba\x49\xac\xd8\x59\x54\xde\x28\xd6\xd2\xca\x47\x6b\x53\xe5\xa3\x1d\xbc\xf2\xd1\x8d\x52\xf9\xcc\x0d\xc6\x3e\x6c\x1d\x08\x73\xbd\x24\x8a\x5e\x07\xfd\x08\xcc\xc1\xdb\x80\x40\x59\xa2\xd0\x32\xd1\xed\x53\x6d\x23\x8d\x61\x3d\x64\x5a\xa7\xdb\xfa\x4f\x76\x62\xae\x43\xb4\x60\xd6\x55\x1d\x66\x2d\x1d\x9c\xeb\x55\x8c\xec\xe1\x7e\xf5\x12\x6e\x2f\x9f\xdd\xcc\xb7\x19\xe6\xee\x1a\x6b\xed\x2d\xa2\x83\x09\x25\x2f\x5c\x38\xd2\xca\x0a\x12\x0f\x5b\xd9\x9c\x7f\x57\x83\x7b\xe4\xbf\xd6\xd0\x22\xdb\x8c\x2d\xf0\xe7\x9d\xc8\x56\xc4\x0c\xd2\x0d\x10\x64\xa4\x67\x77\xbe\xe7\x86\xc7\xd5\xa2\xc4\x6c\x3f\x7c\x3b\xca\x62\x22\x78\x80\xba\x55\x55\x63\x8a\xf3\x04\x74\x25\xfd\xeb\x6c\x48\x0f\x6f\xeb\x4a\x1b\x45\x0c\x6e\xb6\x83\xfa\x68\x43\x4c\xda\x8a\xcd\x8a\x68\xf5\x77\x0f\x1e\x4a\xdc\xd4\xbf\xdc\xd2\xf6\x45\x9e\x3d\xdf\x22\x2d\x18\x6d\x47\x68\x1b\x0a\x5a\x27\xa5\x13\x01\xad\xd2\x3d\xab\xdf\xb9\xa0\x65\xad\xe5\x43\x5b\xd6\x6a\xf1\x56\xbd\xed\xc4\x8e\x23\x67\xc7\xff\x05\x1a\xb7\xc6\xa5\xb9\x31\x2b\x2a\x45\xc0\x37\x77\x60\xd3\x20\xe6\xcd\x0f\x00\x5a\x41\x7b\x20\x6c\x3b\x81\xdb\x05\xdd\x36\xf0\xf6\xac\x71\x44\xd6\x18\x0d\x9d\x05\xe7\x14\x63\xdc\x2e\x0c\x89\xde\x74\x8c\xdf\x3b\x5e\x17\xa2\x7b\xba\xf4\x03\xb1\x1b\xd7\x07\xa8\xac\x56\x70\x1d\xb4\x8a\xf5\xd2\xaf\xbd\x97\xae\x07\x92\xbb\xf8\xed\x10\xdf\xe7\xa8\xbe\xed\x5a\x62\xcf\x37\xe9\x2e\x80\x3a\x7c\xbe\x19\x38\x8d\xe5\x6a\x6e\xb9\xcb\xed\x82\x43\x78\x79\xb9\xcb\xc5\x12\x12\x62\xc2\xec\x63\xed\x60\xe1\x5d\x88\xc0\x99\xff\xf6\x04\x95\x2a\xef\xd2\x7a\x13\xd4\x3c\x22\xd8\x81\x4f\x75\x5b\xe9\x6d\xa8\x6c\xea\xac\x15\x9f\x52\x54\xdb\x36\x33\xde\x90\x0d\x82\x48\xe3\x35\xaa\xd2\x16\x5f\x32\xf9\x39\x44\x51\x6b\xc0\x6b\x8a\xc8\x74\xe5\x1c\xc7\x8e\x52\xdd\x30\xb6\x1b\xda\x7c\x7d\x30\x0c\x48\x1a\x99\x25\x3c\x2e\xb3\x19\x2e\x78\x9c\xc6\x65\x53\x39\x0f\x01\x89\xb4\xd7\x5f\xdd\x16\x7b\x2f\x2b\x43\xf7\x7a\xf9\x2b\xb9\xb6\xea\x77\x1c\xd5\x60\x24\x28\x57\x29\x7a\xa0\x07\xd9\xaf\xb9\xd5\x7c\x58\xf4\xf9\xe0\x2a\xd6\x1a\x5e\xb8\xb6\x0e\x3f\xda\x94\x34\xbc\xfb\xf7\x79\x61\xc3\x45\xb6\x34\xda\x95\x69\x78\xc5\x40\x15\x37\xa8\x38\x99\x39\xd0\xe9\xad\x30\xe4\xda\xce\x81\x09\xb9\x2e\xc1\x0c\xbc\x3c\x8d\xd3\x3c\xe6\x11\x51\x76\x76\x4c\xa3\x0b\xc2\xea\x73\x88\x0a\x57\x40\x23\x92\x6a\xb4\xad\x44\xc0\xc5\x5f\x5f\xba\x77\x11\xc6\x28\xcc\xb4\x4c\x27\x75\x5e\x32\x62\x5d\xd5\xb9\x8a\x0f\x5a\x0a\x20\xc6\x28\xbe\x4e\x0d\x6a\x98\x03\x95\x51\x1a\x8b\xba\x14\xa1\x54\xa6\xc2\xcc\xa0\x50\xf7\x42\x2a\xc0\x6b\x12\x27\x11\x4e\x81\x0b\x70\xe5\x7c\xd9\x1a\x2a\x8e\x57\x68\x49\xb1\xda\x57\xfb\x93\x47\x02\xa9\x46\x65\x95\x97\x2e\x1a\xa2\xdc\x39\x9e\x13\xb8\x8c\xb7\x97\xcb\xb3\xe2\xe1\xe5\xe5\xa5\xfe\x14\x55\xbc\xf0\x9d\x21\xe2\x1f\x11\x26\xf1\xf6\x77\x93\xaa\x68\xd9\xef\xdd\xee\xa4\x03\x25\x02\x48\xa4\x25\xac\xd1\x9f\x05\x22\x03\x69\x03\x2b\xaa\xfd\x18\xc1\xec\x00\x27\x75\xba\x2e\x60\xa0\x3d\xe1\xa1\x2b\x2f\xb9\x0c\xa4\x7c\xba\x26\xea\x72\xda\xe9\x53\xb5\xef\xca\x73\xe5\xec\x23\x6e\xe1\x29\x4c\x02\x29\x27\x40\x04\x6b\x95\xb9\x22\x51\x8a\x56\x6a\x4d\x54\xc7\x2c\xfc\xec\x97\xaf\x8a\x2c\x31\x31\x96\xa4\xaf\x38\x43\x36\x05\xa9\x80\x7b\x19\xaf\x8d\x6b\xc0\x38\x31\xdb\xa9\x6d\x2b\x0f\xb6\x77\xd6\xd2\x84\xc4\xb8\x16\xbb\x20\x10\x12\x0d\x09\xaa\x98\x6b\x9b\xb7\xda\x09\xd2\x88\xf0\x99\x47\x11\xac\xcb\x75\xf6\xd1\x8d\x6c\x36\x94\x4b\xb3\x12\xd1\x7a\x88\x66\x8d\x77\x10\xa3\x7e\x75\xd7\xdb\x5b\x8f\xd2\x5c\xf1\xb0\x40\x5d\xa7\x66\xef\x60\x6d\x84\xe9\x9e\x00\x2e\x56\xd5\x3d\xf6\xb8\xcd\x03\x6d\x40\x28\x12\x4d\xdb\xd1\xf7\x5a\x1d\x36\x26\xac\x88\x60\x2b\x08\xb8\xd2\x06\x86\x1b\x31\xf5\x3d\x5e\xf5\xda\x74\x5b\x11\x21\x24\xe0\x75\x12\x71\xca\x8d\x77\xc1\x13\x98\x43\x7c\x4e\x2e\x83\x81\xee\x2b\x9b\xeb\x38\xf7\x6d\xb7\x03\xf3\xd4\xd9\xa3\xdd\x2d\x67\x1c\x93\x73\x8d\xd6\x7f\xcb\x79\xf9\x37\x32\xfc\x68\x76\x95\xd6\xb8\x13\xa8\x00\x2f\xfc\x63\x19\x58\x22\x3a\xd7\x46\xa5\xd4\xa4\xca\x6a\x14\x2e\x71\x72\x99\xa7\xb6\xab\x01\x4f\x8a\xa7\x3f\xcc\x9e\x38\xb5\x3f\x80\x90\xc6\x1d\xe7\x96\x0a\x9f\x68\x93\x0b\x7d\x0b\x31\x12\xa1\x1d\x2a\x9c\xbc\x53\x08\x85\x9a\xa2\xcf\x4f\x1e\xc8\x4b\x8f\x6a\x42\x43\xb8\xa8\xb0\xa2\xb5\x7d\x83\x06\x38\x9b\xba\x4b\x85\x29\x24\x11\x11\xdf\x70\xe6\x6c\xfc\xc8\x05\x7b\xe4\xfe\xf2\xe4\x09\xdf\x14\xc3\xe9\x47\x35\x74\x15\x7f\x4b\x1a\x3b\x85\x75\x6a\x3f\x3f\x2f\xa1\xe3\xbb\x3f\xe5\x6c\xea\x06\xb4\xe3\xcd\x38\xf3\xff\xdb\x01\xa7\x19\x51\x7f\x5b\xef\x85\x86\x86\x2f\xdd\x93\xa7\xb5\x1a\xa3\x72\xf0\x5e\xc0\xfc\x37\x00\x00\xff\xff\x66\x05\x16\x0d\x41\x57\x00\x00")

func openapiYamlBytes() ([]byte, error) {
	return bindataRead(
		_openapiYaml,
		"openapi.yaml",
	)
}

func openapiYaml() (*asset, error) {
	bytes, err := openapiYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "openapi.yaml", size: 22337, mode: os.FileMode(493), modTime: time.Unix(1717473157, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"openapi.yaml": openapiYaml,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}

var _bintree = &bintree{nil, map[string]*bintree{
	"openapi.yaml": &bintree{openapiYaml, map[string]*bintree{}},
}}

// RestoreAsset restores an asset under the given directory
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	err = os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
	if err != nil {
		return err
	}
	return nil
}

// RestoreAssets restores an asset under the given directory recursively
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}
