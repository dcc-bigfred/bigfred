package domain

import gonanoid "github.com/matoous/go-nanoid/v2"

// idAlphabet excludes '-' and '_' so V-{suffix} / T-{suffix} split unambiguously.
const idAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZabcdefghijkmnopqrstuvwxyz"

const idSuffixLen = 10

// MaxCatalogueIDRetries is the upper bound on insert retries after a PK collision.
const MaxCatalogueIDRetries = 5

// NewVehicleID returns a fresh local catalogue id (V-{nanoid10}).
func NewVehicleID() (VehicleID, error) {
	suffix, err := gonanoid.Generate(idAlphabet, idSuffixLen)
	if err != nil {
		return "", err
	}
	return VehicleID(vehicleIDPrefix + suffix), nil
}

// NewTrainID returns a fresh local catalogue id (T-{nanoid10}).
func NewTrainID() (TrainID, error) {
	suffix, err := gonanoid.Generate(idAlphabet, idSuffixLen)
	if err != nil {
		return "", err
	}
	return TrainID(trainIDPrefix + suffix), nil
}
