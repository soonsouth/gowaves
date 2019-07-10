package state

const (
	// Default values.
	// Cache parameters.
	// 500 MiB.
	DefaultCacheSize = 500 * 1024 * 1024

	// Bloom filter parameters.
	// Number of elements in Bloom Filter.
	DefaultBloomFilterSize = 2e8
	// Acceptable false positive for Bloom Filter (0.01%).
	DefaultBloomFilterFalsePostiiveProbability = 0.0001

	// Db parameters.
	DefaultWriteBuffer         = 16 * 1024 * 1024
	DefaultCompactionTableSize = 4 * 1024 * 1024
	DefaultCompactionTotalSize = 20 * 1024 * 1024

	// Block storage parameters.
	// DefaultOffsetLen is the amount of bytes needed to store offset of transactions in blockchain file.
	DefaultOffsetLen = 8
	// DefaultHeaderOffsetLen is the amount of bytes needed to store offset of headers in headers file.
	DefaultHeaderOffsetLen = 8
)