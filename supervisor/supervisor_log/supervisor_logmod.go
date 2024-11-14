package supervisor_log

import (
	"emu/params"
	"io"
	"log"
	"os"
)

type SupervisorLog struct {
	Slog *log.Logger
	f    *os.File
}

func NewSupervisorLog() *SupervisorLog {
	writer1 := os.Stdout

	dirPath := params.LogWritePath
	err := os.MkdirAll(dirPath, os.ModePerm)
	if err != nil {
		log.Panic(err)
	}
	writer2, err := os.OpenFile(dirPath+"/Supervisor.log", os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		log.Panic(err)
	}
	pl := log.New(io.MultiWriter(writer1, writer2), "Supervisor: ", log.Lshortfile|log.Ldate|log.Ltime)
	return &SupervisorLog{
		Slog: pl,
		f:    writer2,
	}
}

func (s *SupervisorLog) Sync() {
	s.f.Sync()
}
