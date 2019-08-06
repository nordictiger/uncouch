package couchdbfile

import (
	"encoding/binary"
	"fmt"
	"github.com/pipedrive/uncouch/couchbytes"
	"github.com/pipedrive/uncouch/termite"
	"io"
)

// TreeState is subset of data in db header we care for our purposes
type TreeState struct {
	Offset int64
	Size   int32
}

// DbHeader is subset of data db header we care for our purposes
type DbHeader struct {
	DiskVersion  uint8
	UpdateSeq    int32
	IDTreeState  TreeState
	SeqTreeState TreeState
}

// findHeader tries to locate DB Header from provided input.
// It returns offset if header was found.
func (dbh *DbHeader) findHeader(input io.ReadSeeker, size int64) (offset int64, err error) {
	latestBlockIndex := size / couchbytes.BlockAlignment
	for {
		if latestBlockIndex < 0 {
			// We reached beginning of the file and didn't find DB header block, something must be wrong
			err = fmt.Errorf("Could not find DB Header block in the file")
			slog.Error(err)
			return -1, err
		}
		offset, err = input.Seek(latestBlockIndex*couchbytes.BlockAlignment, io.SeekStart)
		if err != nil {
			slog.Error(err)
			return -1, err
		}
		var headerFlag uint8
		err = binary.Read(input, binary.BigEndian, &headerFlag)
		if err != nil {
			slog.Error(err)
			return -1, err
		}
		switch headerFlag {
		case 0:
			latestBlockIndex--
		case 1:
			offset++
			return offset, err
		default:
			err := fmt.Errorf("Unknown DB Header starting byte %v", headerFlag)
			slog.Error(err)
			return -1, err
		}
	}
}

// readFromTermite reads header structure out of Termite structure
func (dbh *DbHeader) readFromTermite(t *termite.Termite) error {
	if string(t.Children[0].T.Binary) != "db_header" {
		err := fmt.Errorf("Term header is \"%s\". Expecting \"db_header\"", string(t.Children[0].T.Binary))
		slog.Error(err)
		return err
	}
	dbh.DiskVersion = uint8(t.Children[1].T.IntegerValue)
	dbh.UpdateSeq = int32(t.Children[2].T.IntegerValue)

	if len(t.Children[4].Children) >= 3 {
		dbh.IDTreeState.Offset = int64(t.Children[4].Children[0].T.IntegerValue)
		dbh.IDTreeState.Size = int32(t.Children[4].Children[2].T.IntegerValue)
	} else {
		dbh.IDTreeState.Offset = 0
		dbh.IDTreeState.Size = 0
	}

	if len(t.Children[5].Children) >= 3 {
		dbh.SeqTreeState.Offset = int64(t.Children[5].Children[0].T.IntegerValue)
		dbh.SeqTreeState.Size = int32(t.Children[5].Children[2].T.IntegerValue)
	} else {
		dbh.SeqTreeState.Offset = 0
		dbh.SeqTreeState.Size = 0
	}

	return nil
}
