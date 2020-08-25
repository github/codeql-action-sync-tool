package version

import log "github.com/sirupsen/logrus"

var version = "development"
var commit = "0000000000000000000000000000000000000000"

func Version() string {
	return version
}

func Commit() string {
	return commit
}

func LogVersion() {
	log.Infof("Starting CodeQL Action sync tool version %s...", Version())
}
