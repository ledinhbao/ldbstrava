package ldbstrava

import "log"

func getCallbackURLOrPanic(panic bool) string {
	if config.URLCallbackHost == "" {
		if panic {
			log.Panic("Failed to obtain URLCallbackHost for Strava Module.")
		}
	}
	return config.URLCallbackHost + config.PathPrefix + config.PathSubcription
}
