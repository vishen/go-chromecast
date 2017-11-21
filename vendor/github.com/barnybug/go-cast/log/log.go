package log

import "log"

var Debug = false

func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
}

func Println(v ...interface{}) {
	if Debug {
		log.Println(v...)
	}
}

func Printf(format string, v ...interface{}) {
	if Debug {
		log.Printf(format, v...)
	}
}

func Fatalln(v ...interface{}) {
	log.Fatalln(v...)
}

func Fatalf(format string, v ...interface{}) {
	log.Fatalf(format, v...)
}

func Errorln(v ...interface{}) {
	log.Println(v...)
}

func Errorf(format string, v ...interface{}) {
	log.Printf(format, v...)
}
