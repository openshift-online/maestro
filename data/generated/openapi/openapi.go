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

var _openapiYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xec\x5c\x5f\x8f\xdb\xb8\x11\x7f\xdf\x4f\x31\x40\x5b\x38\x39\xec\xda\x4e\xef\x0a\xb4\x46\x72\x40\x72\xbd\x14\x77\xc8\x25\x69\x36\x69\x1f\x8a\xc2\x4b\x93\x23\x8b\x89\x44\x2a\x24\xb5\x59\xa7\xed\x77\x2f\x48\xea\xbf\x25\xad\xec\xf3\xc6\xca\x9e\xf3\x92\x35\x35\x33\x9c\x21\x67\x7e\x1a\x0e\xc7\x96\x09\x0a\x92\xf0\x05\x7c\x3b\x9d\x4f\xe7\x67\x5c\x04\x72\x71\x06\x60\xb8\x89\x70\x01\x31\x41\x6d\x94\x84\x4b\x54\xd7\x9c\x22\x3c\x7d\xfd\xd3\x19\x00\x43\x4d\x15\x4f\x0c\x97\xa2\x8b\xe4\x1a\x95\x76\x8f\xe7\xd3\xf9\xf4\xd1\x99\x46\x65\x47\xac\xe4\x0b\x48\x55\xb4\x80\xd0\x98\x64\x31\x9b\x45\x92\x92\x28\x94\xda\x2c\xfe\x3c\x9f\xcf\xcf\x00\x1a\xd2\x69\xaa\x14\x0a\x03\x4c\xc6\x84\x8b\x3a\xbb\x5e\xcc\x66\x24\xe1\x53\x6b\x82\x0e\x79\x60\xa6\x54\xc6\xdb\x22\x7e\x21\x5c\xc0\x83\x44\x49\x96\x52\x3b\xf2\x10\xbc\x36\xed\xc2\xb4\x21\x6b\xbc\x4d\xe4\xa5\x21\x6b\x2e\xd6\xb9\xa0\x84\x98\xd0\xd9\x66\x25\xcc\xb2\x05\x99\x5d\x3f\x9a\x29\xd4\x32\x55\x14\xdd\x43\x80\x35\x1a\xff\x07\x80\x4e\xe3\x98\xa8\xcd\x02\xde\xa0\x49\x95\xd0\x40\x20\xe2\xda\x80\x0c\xa0\x60\xca\x49\x91\xa6\x8a\x9b\x4d\xce\x6a\xd5\x7e\x86\x44\xa1\x5a\xc0\xbf\xfe\x9d\x0d\x2a\xd4\x89\x14\x3a\x9f\xc9\xfe\x9b\xfc\x71\x3e\x9f\x94\x1f\x1b\x26\x3c\x85\x9f\x2f\x5f\xbd\x04\xa2\x14\xd9\x54\x67\x05\xb9\x7a\x8f\xd4\xe8\x0a\x1f\x95\xc2\xa0\x30\x55\x51\x00\x24\x49\x22\x4e\x89\x15\x36\x7b\xaf\xa5\xa8\x3f\x05\xd0\x34\xc4\x98\x34\x47\x01\x7e\xaf\x30\x58\xc0\xe4\x77\x33\x2a\xe3\x44\x0a\x14\x46\xcf\x3c\xad\x9e\xbd\xc9\x74\x78\xc1\xb5\x99\x94\x76\x7c\x37\x7f\xd4\x63\x47\x6a\x42\x30\xf2\x03\x0a\xe0\x1a\xb8\xb8\x26\x11\x67\xc7\x50\xfe\x47\xa5\xa4\xaa\x69\xfd\x6d\xb7\xd6\xef\x04\x49\x4d\x28\x15\xff\x8c\x0c\x8c\x84\x04\x55\x20\x55\x0c\x32\x41\xe5\xd4\x1a\x83\x05\x7f\xea\xf3\x9f\x77\x02\x6f\x12\xa4\x06\x19\xa0\xe5\x03\x49\x5d\xac\x1e\x7f\xed\x13\xa2\x48\x8c\x26\x83\x1b\x70\xf1\xd2\xc6\x5c\xd2\xcd\x12\xb2\xc6\xc9\x50\x62\xcd\x3f\xef\x40\x8c\x44\xd1\x70\x30\xb9\x54\x0c\xd5\xb3\xcd\x60\xfa\x80\x63\xc4\xb4\x27\x4f\x2c\x8a\x36\xe1\xe5\x07\x85\xc4\x20\x10\x10\xf8\xa9\x88\xf1\xdd\x80\xe5\x63\x8a\xda\x3c\x93\xac\x42\x57\xf3\x84\x3c\x6a\x81\x11\x43\x0a\x12\xcb\xc7\x15\xb2\x05\x18\x95\xe2\x59\x8f\x4b\xf4\x3b\x44\xbb\x3b\x0c\x41\x91\x49\x2f\x34\xf6\x40\x8a\x5f\xb3\xa3\x38\x72\x53\x77\x87\x23\x3d\x51\xf8\x0f\x8b\x76\x4e\x05\x1f\x85\x7a\x3c\x61\x78\x02\xee\x23\x5a\xf0\x97\x6e\x0b\x8a\x70\x25\x91\x42\xc2\x36\x80\x37\x5c\x1f\xe7\x7d\xbf\xd3\x0b\xe7\xa9\x80\xb4\xeb\x9d\x03\xd4\x86\xac\xcd\xc8\x4c\x88\x4d\x98\x3b\x8e\x49\x9d\xa9\xe0\xec\x3f\x9c\xfd\xaf\x3b\x1f\xfc\x1b\x1a\x20\xa2\x4c\xc7\x56\x1b\x28\xc2\xe2\x6e\x32\xc1\xc2\x21\x02\x99\x0a\x56\x9b\xf0\x8b\x2e\x5d\x2b\xf6\x9d\x00\xe4\x38\x16\x7c\xd7\x6d\xc1\x4b\x59\x7a\xe7\x27\x6e\x42\xd0\x09\x52\x1e\x70\x64\xc0\xd9\xd7\x82\x26\x63\x4d\x5f\x13\x62\x68\xb8\x05\x0a\xef\x12\xe6\xb2\x38\x71\x47\x29\x9c\x97\xcf\xca\x7d\x1d\x59\x2a\xf7\xda\xae\xca\x1b\x6f\x46\x7f\x5a\x37\x04\xe7\xd2\xcc\x5a\x9d\x52\x8a\x5a\x07\x69\x14\x6d\x46\x03\x78\xa7\x64\xef\x0b\x6b\x7d\xc2\xea\x51\x18\x71\x0f\x33\xd6\xad\x77\x8c\x03\x1e\x9b\xa5\x8e\x22\x43\xb5\xda\x46\x68\x70\xeb\x6d\xf3\x57\x37\x0c\x64\xcf\x97\x4d\x1b\x2c\xf7\xb8\x68\x59\x3e\x70\xd3\x76\xc0\xf2\x09\x19\xbf\xbc\xd6\x27\x64\x1c\x81\x11\xbb\x21\x8c\x8b\xa1\x11\x21\x4c\xb3\x16\x7b\x6b\x41\x93\xb3\xbe\xc3\xf3\xc5\x2a\x15\x2c\xda\xef\x3a\x05\x32\xde\x3b\x3d\x4b\x77\xde\xaa\xf8\xc9\xc7\x70\xb9\xf2\xcc\x69\x72\xba\x62\x19\x05\x44\x7d\x95\x67\xd4\xdf\xea\x15\xcb\x6d\xa8\xb4\x6b\x65\xcf\x43\xc2\x17\x2c\xf0\x65\x33\x8e\xa4\xce\xe7\x81\xe8\x04\x42\x23\xb0\x60\x60\x9e\x94\xf9\xcf\xfd\x49\x97\xbe\x66\x40\x6d\xcf\x94\xa8\x14\x3a\x8d\x0b\x39\xc3\x52\xa4\x82\xe9\x8b\xe6\x46\xf9\xac\xc7\x4c\x8a\x7e\xc8\x74\x38\xa5\x43\xa3\x40\xa2\x7b\x13\xbd\x3b\x26\x44\x3b\xa6\x44\x3b\x27\x45\xbb\xa7\x45\x07\xef\x3d\xc9\xa3\x7d\x37\x88\xb9\xed\xe2\x22\x8f\xdf\xb1\x5c\x58\xe4\xfa\x7c\x8d\xbd\x27\x4d\xdd\x4f\x45\xb7\x13\x84\xef\x63\x41\x4f\x25\xbf\x08\xd7\xaf\xac\x92\x3f\xbc\xf7\xa4\x01\x73\xc7\x31\xa9\x33\x29\x1c\x76\x42\x2d\x12\xb3\xbb\x3f\x9a\x16\x0e\x71\xe4\x33\x69\x2b\xf6\x9d\x00\x64\x8c\xa7\xd1\xc2\x3b\x4f\xc7\xd0\x83\x17\xeb\xfb\x7b\x4f\xee\x26\x85\xcb\x7b\x4f\xe8\x48\x53\xb9\x83\xf4\x9e\x14\x38\x37\x96\xde\x93\x53\xb2\x37\x06\xad\x4f\x58\x3d\x0a\x23\xee\x61\xc6\xda\xdd\x7b\x32\x8a\x0c\xf5\xf6\xde\x93\xfd\x5e\x36\x3b\xf6\x9e\x94\xe5\x83\x53\xef\xc9\x09\x19\x0f\x6b\xc1\x3d\x40\xc6\x3d\x7b\x4f\x46\x82\x30\x7b\xde\xa9\x94\x4f\x2c\x5b\x8e\x3b\x97\x56\x7e\x0e\x2c\x19\xf0\x64\x52\xcd\x26\x41\xff\x1d\xe2\xb3\x8a\xde\xb8\x80\x95\x23\xcb\x06\xfd\x87\xe7\x52\xc5\xc4\x2c\xe0\xe7\x7f\xbe\x3d\xcb\x0d\xcc\x84\xbe\x72\xb7\x20\x6f\x30\x40\x85\x82\x62\x5d\xba\xbf\x22\xc9\x86\x12\x65\x5d\xdd\xf0\x2a\xce\x71\x56\x5d\x27\xcf\xa4\x8d\xe2\x62\x5d\x0c\x7f\xe0\xe2\x76\xa2\xd0\x2e\x50\x1f\xd1\x0b\x5e\x56\x7a\x07\xea\x36\x68\xe2\x84\xac\x71\x9b\x88\x0b\x83\xeb\x8a\x27\x69\xfe\x79\x00\x95\x91\x86\x44\xb7\x91\x15\x27\x8b\xca\x1b\xc5\x6a\x5a\xf9\x68\x75\xaa\x7c\xb4\x93\x57\x3e\xba\x59\x2a\x9f\xb9\xc1\xd8\x87\xad\x73\xc2\x5c\x2e\x89\xa2\x57\x41\xbf\x07\xe6\xce\xdb\x70\x81\xb2\x45\xa1\x65\xa1\xdb\x97\xda\x46\x1a\xc3\x7a\xc8\xb4\x2e\xb7\xb5\x9f\x6c\xc5\x5c\x07\x69\x81\xac\xcb\xba\x9b\xb5\x30\x38\xd3\xab\x3e\xb2\x83\xf9\xd5\x4b\xb8\x9d\x6c\x76\x2b\xdf\xa6\x98\xbb\x6b\xac\x8d\xb7\x90\x0e\x06\x94\xbc\x71\xe1\x48\x3b\x2b\x48\x3c\x6c\x67\x73\xfc\x5d\x0e\xe6\xc8\x7f\xad\xa1\x85\xb6\x19\x5b\xe0\xeb\x9d\xc8\x96\xc4\x0c\x92\x0d\x10\x64\xa0\x67\x4f\xbe\x17\x86\xc7\xd5\xa6\xc4\xec\x3c\x7c\x18\x61\x59\x16\x77\x18\x61\x31\x11\x3c\x40\xdd\x2a\xaa\xb1\x5f\x00\x6b\x25\xd3\x64\xa9\x1a\x0e\xd2\xcb\xe2\x95\x5d\x4a\xff\x3a\x1d\xc2\xe1\xd7\x6a\xa9\x8d\x22\x06\xd7\x9b\x41\x3c\xda\x10\x93\xb6\xc6\x46\x85\xb4\xfa\xbb\x0b\xf7\x25\x6e\xeb\x5f\xae\x69\xfb\x22\xd1\x8e\x6f\xb1\x96\x18\x69\x8f\x90\x36\xc7\x69\x5d\x94\x4e\x0f\x68\xa5\xee\xd9\xfd\xce\x0d\x2d\x7b\x3d\xef\xdb\xb6\x56\x9b\xc7\xea\x63\x27\x74\xfe\x4d\xa0\x33\x1a\xc2\x88\x21\x83\x40\x30\x8f\xc8\x5f\xe3\x94\x07\x43\xf1\x5c\x99\x25\x95\x22\xe0\xeb\x3b\xd0\x69\x10\xe6\xe7\xa5\x8f\xd6\x70\xd9\x33\x60\x3a\x43\xa6\x2b\x68\xda\xc2\xa6\xc7\x21\x22\xb2\xc2\x68\xe8\x2a\x38\xa3\x18\xe3\x76\x63\x48\xf4\xba\x63\xfe\xde\xf9\xba\x62\xa9\x87\xa5\xdf\x6b\xbb\x23\x6a\x0f\x91\xd5\xde\xb5\xbd\x76\xb1\xde\xf4\xb6\xf3\xd6\xf5\xb8\xe4\xb6\xff\x76\x90\xef\x72\x49\xd1\x76\x21\xb3\xe3\x3b\x7c\xdb\x81\x3a\x6c\xbe\xdd\x71\x1a\xdb\xd5\x2c\x36\x94\x07\x25\xe7\xe1\xe5\xb5\x36\x17\x0b\x48\x88\x09\xb3\x8f\xb5\x92\xca\xdb\x10\x81\x33\xff\xbd\x11\x2a\x55\xce\xd2\x7a\x07\xd6\x2c\x8e\x6c\xb9\x4f\xf5\x40\xed\x75\xa8\x1c\x67\xad\x16\x1f\x53\x54\x9b\x36\x35\x5e\x93\x35\x82\x48\xe3\x15\xaa\x52\x17\xdf\x2c\xfa\x29\x44\x51\x1b\xc0\x1b\x8a\xc8\x74\xa5\x82\x65\x67\xa9\x1e\x95\xdb\x15\x6d\xbe\xb8\x18\x06\x24\x8d\xcc\x02\x1e\x95\x79\x14\x17\x3c\x4e\xe3\x72\xa8\x5c\x87\x80\x44\xda\xcb\xaf\x16\x04\xbc\x95\x95\xa9\x7b\xad\xfc\x85\xdc\x58\xf1\x5b\x86\x6a\x30\x12\x94\xeb\x91\xdd\xd3\x82\xec\x77\xec\x6a\x36\xcc\xfb\x6c\x70\xbd\x7a\x0d\x2b\xdc\x58\x87\x1d\x6d\x42\x1a\xd6\xfd\xf7\xa2\xd0\xe1\x32\xdb\x1a\xed\x1a\x54\xbc\x60\xa0\x8a\x1b\x54\x9c\x4c\x9d\xd3\xe9\x8d\x30\xe4\xc6\xae\x81\x09\xb9\x2e\x9d\x19\x78\x59\x87\xd4\x3c\xe6\x11\x51\x76\x75\x4c\x83\x05\x61\xf9\x29\x44\x85\x4b\xa0\x11\x49\x35\xda\x51\x22\xe0\xf2\xef\x2f\xdc\xbb\x08\x63\x14\xe6\xbc\x4c\x64\x75\xde\x2c\x63\x4d\xd5\xb9\x88\xf7\x5a\x0a\x20\xc6\x28\xbe\x4a\x0d\x6a\x98\x01\x95\x51\x1a\x8b\x3a\x15\xa1\x54\xa6\xc2\x4c\xa1\x10\xf7\x5c\x2a\xc0\x1b\x12\x27\x11\x9e\x03\x17\xe0\x1a\x19\xb3\x3d\x54\x1c\xaf\xd1\x82\x62\x95\x57\xfb\x9a\x2b\x81\x54\xa3\xb2\xc2\x4b\x13\x0d\x51\xae\x82\xe9\x08\xae\xe2\xcd\xd5\xe2\xac\x78\x78\x75\x75\xa5\x3f\x46\x15\x2b\x3c\x33\x44\xfc\x03\xc2\x24\xde\xfc\x61\x52\x25\x2d\xf9\xde\x6e\x2f\x3a\x50\x22\x80\x44\x5a\xc2\x0a\x7d\x15\x14\x19\x48\x1b\x58\x51\xed\x67\x18\xa6\x7b\x18\xa9\xd3\x55\xe1\x06\xda\x03\x1e\xba\xc6\x9a\xab\x40\xca\x27\x2b\xa2\xae\xce\x3b\x6d\xaa\xf2\x2e\x3d\x56\x4e\x3f\xe0\x06\x9e\xc0\x24\x90\x72\x02\x44\xb0\x56\x9a\x6b\x12\xa5\x68\xa9\x56\x44\x75\xac\xc2\x4f\x7e\xfb\xaa\x9e\x25\x26\xc6\x82\xf4\x35\x67\xc8\xce\x41\x2a\xe0\x9e\xc6\x4b\xe3\x1a\x30\x4e\xcc\xe6\xdc\x8e\x95\x25\xfd\xad\xbd\x34\x21\x31\x6e\xc4\x6e\x08\x84\x44\x43\x82\x2a\xe6\xda\x66\xcc\x76\x81\x34\x22\x7c\xe2\x51\x04\xab\x72\x9f\x7d\x74\x23\x9b\x0e\xc5\xd2\xac\x39\xb6\x1e\xa2\xd9\xe0\x1d\xc4\xa8\xdf\xdd\xd5\xe6\xe0\x51\x9a\x0b\x1e\x16\xa8\xab\xd4\xec\x1c\xac\x8d\x30\xdd\xd1\x81\x8b\x5d\x75\x8f\xbd\xdf\xe6\x81\x36\x20\x14\x89\xa6\xed\xde\xf7\x4a\xed\x37\x27\x2c\x89\x60\x4b\x08\xb8\xd2\x06\x86\x2b\x71\xee\x39\x5e\xf6\xea\x74\xa8\x88\x10\x12\xf0\x26\x89\x38\xe5\xc6\x9b\xe0\x01\xcc\x79\x7c\x0e\x2e\x83\x1d\xdd\xf7\x74\xd7\xfd\xdc\x8f\x1d\xc6\xcd\x53\xa7\x8f\x76\xf7\xbb\x71\x4c\x2e\x34\x5a\xfb\x2d\xe6\xe5\xdf\x45\xf1\xb3\xd9\x5d\x5a\xe1\x56\xa0\x02\x3c\xf7\x8f\x65\x60\x81\xe8\x42\x1b\x95\x52\x93\x2a\x2b\x51\xb8\xc4\xc9\x65\x9e\xda\xee\x06\x3c\x2e\x9e\x7e\x3f\x7d\xec\xc4\x7e\x0f\x42\x1a\x57\xc8\x2e\x05\x3e\xd6\x26\x27\xfa\x06\x62\x24\x42\x3b\xaf\x70\xf4\x4e\x20\x14\x62\x0a\x9e\x1f\xbd\x23\x2f\xbc\x57\x13\x1a\xc2\x65\x05\x15\xad\xee\x6b\x34\xc0\xd9\xb9\xbb\x4e\x39\x87\x24\x22\xe2\x01\x67\x4e\xc7\x0f\x5c\xb0\x87\xee\x2f\x0f\x9e\xf0\xa0\x98\x4e\x3f\xac\x79\x57\xf1\xb7\xa4\xb1\x13\x58\x87\xf6\x8b\x8b\xd2\x75\x3c\xfb\x13\xce\xce\xdd\x84\x76\xbe\x29\x67\xfe\x7f\x3b\xe1\x79\x06\xd4\xdf\xd4\xb9\xd0\xd0\xf0\x85\x7b\xf2\xa4\xd6\x5d\x55\x4e\xde\xeb\x30\xff\x0f\x00\x00\xff\xff\x44\x7b\xe9\xbf\x3b\x58\x00\x00")

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

	info := bindataFileInfo{name: "openapi.yaml", size: 22587, mode: os.FileMode(493), modTime: time.Unix(1737424969, 0)}
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
