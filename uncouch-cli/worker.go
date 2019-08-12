package main

import (
	"errors"
	"github.com/pipedrive/uncouch/couchdbfile"
	"github.com/pipedrive/uncouch/tar"
	"io"
	"strconv"
	"sync"
)

type muMap map [string]mapV

type mapV struct {
	Mu *sync.Mutex
	ActiveQ uint8
}

func (m muMap) AddQ(f string) {
	m[f] = mapV{Mu: m[f].Mu, ActiveQ: m[f].ActiveQ+1}
}

func (m muMap) SubstractQ(f string) {
	m[f] = mapV{Mu: m[f].Mu, ActiveQ: m[f].ActiveQ-1}
}

func (m muMap) Get(f string, mapMutex *sync.Mutex) (*sync.Mutex) {
	mapMutex.Lock()
	defer mapMutex.Unlock()

	mv, found := m[f]
	if found{
		m.AddQ(f)
		return mv.Mu
	}
	var newMutex sync.Mutex
	var newMV mapV
	newMV.Mu = &newMutex
	newMV.ActiveQ = 1
	m[f] = newMV

	return &newMutex
}

func (m muMap) Delete(f string, mapMutex *sync.Mutex) {
	mapMutex.Lock()
	defer mapMutex.Unlock()
	if m[f].ActiveQ == 1 {
		delete(m, f)
	} else {
		m.SubstractQ(f)
	}

}

func createWorkers(workersQ uint64, filesChan chan tar.UntarredFile, writesChan chan FileContent, dstFolder string, wgp *sync.WaitGroup, oksChan *[]string, errorsChan *[]error) () {
	wgp.Add(int(workersQ))
	for i := uint64(0); i < workersQ; i++ {
		log.Info("Starting worker: " + strconv.Itoa(int(i)))
		go worker(filesChan, writesChan, dstFolder, wgp, oksChan, errorsChan, i)
	}
	return
}

func worker(filesChan chan tar.UntarredFile, writesChan chan FileContent, dstFolder string, wgp *sync.WaitGroup, oksChan *[]string, errorsChan *[]error, i uint64) () {
	for {
		current, more := <- filesChan
		if !more {
			log.Info("Finalizing worker: " + strconv.Itoa(int(i)))
			wgp.Done()
			return
		}

		// actual processing
		var err error
		var file FileContent

		if current.Input == nil {
			file, err = processFiles(current.Filepath, dstFolder)
		} else {
			file, err = processAll(current.Input, current.Filepath, current.Size, dstFolder)
		}

		if err != nil {
			*errorsChan = append(*errorsChan, err)
			slog.Error(err)
		}
		*oksChan = append(*oksChan, current.Filepath)
		writesChan <- file
	}
}

func processAll(memoryReader io.ReadSeeker, filename string, n int64, dstFolder string) (FileContent, error) {
	var file FileContent

	cf, err := couchdbfile.New(memoryReader, n)
	if err != nil {
		slog.Error(errors.New("Error in file: " + filename))
		slog.Error(err)
		return file, err
	}

	filename = createOutputFilename(filename, dstFolder)

	file = FileContent{cf, filename}

	return file, nil
}

func processFiles(filename, dstFolder string) (FileContent, error) {
	file, err := auxDataFunc(filename, dstFolder)
	if err != nil {
		slog.Error(err)
	}
	return file, err
}

func createWriters(writersQ uint64, writesChan chan FileContent, wgw *sync.WaitGroup, woksChan *[]string, werrorsChan *[]error) () {
	wgw.Add(int(writersQ))
	m := make(muMap)
	var mapMutex sync.Mutex

	for i := uint64(0); i < writersQ; i++ {
		log.Info("Starting writer: " + strconv.Itoa(int(i)))
		go writer(writesChan, wgw, woksChan, werrorsChan, i, m, &mapMutex)
	}
	return
}

func writer(writesChan chan FileContent, wgw *sync.WaitGroup, woksChan *[]string, werrorsChan *[]error, i uint64, m muMap, mMutex *sync.Mutex) () {
	for {
		current, more := <- writesChan
		if !more {
			log.Info("Finalizing writer: " + strconv.Itoa(int(i)))
			wgw.Done()
			return
		}

		mu := m.Get(current.Filename, mMutex)
		// actual processing
		newFilename, err := current.mergeWriteData(mu)

		if err != nil {
			*werrorsChan = append(*werrorsChan, err)
			slog.Error(err)
		}
		m.Delete(current.Filename, mMutex)
		*woksChan = append(*woksChan, newFilename)
	}
	// fmt.Println("Finalizing writer: " + strconv.Itoa(i))
}