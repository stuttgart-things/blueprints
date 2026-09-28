package clustersecrets

import "strings"

// SopsEncryptArgs is the sops invocation for one manifest. Recipients are
// passed on the command line instead of via .sops.yaml: sops looks for its
// config relative to the working directory, and the file names inside the
// container would not match the path_regex rules anyway.
func SopsEncryptArgs(recipients []string, in, out string) []string {
	return []string{
		"sops", "--encrypt",
		"--age", strings.Join(recipients, ","),
		"--encrypted-regex", EncryptedRegex,
		"--input-type", "yaml", "--output-type", "yaml",
		"--output", out, in,
	}
}

// SopsDecryptArgs decrypts in to out with the key in SOPS_AGE_KEY(_FILE).
// The format follows the file extension, so ref+sops:// also reads
// *.enc.json files.
func SopsDecryptArgs(in, out string) []string {
	return []string{"sops", "--decrypt", "--output", out, in}
}
