package config

type Config struct {
	LocalTransfers map[int]*TransferTarget
}

type TransferTarget struct {
	Target string
}
