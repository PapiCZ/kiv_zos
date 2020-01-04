package tests

import (
	"encoding/hex"
	"github.com/PapiCZ/kiv_zos/vfs"
	"math/rand"
	"os"
	"path/filepath"
)

//func TempFileName(prefix, suffix string) string {
//	randBytes := make([]byte, 16)
//	rand.Read(randBytes)
//	return filepath.Join(os.TempDir(), prefix+hex.EncodeToString(randBytes)+suffix)
//}
//
//func testSuperblockWriteToDrive() {
//	path := vfs.PrepareDriveFile(TempFileName("", ""), 1000)
//	drive := vfs.NewDrive(path)
//}
