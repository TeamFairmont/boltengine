#security

The package provides the following public functions:
* AuthenticateGroup- Takes a group name (string) and key (string), plus a pointer to cfg.Security.Groups.  Returns a boolean indicating whether or not the group and key match a pair of values within the config's Groups.
* EncodeHMAC- Takes a group's hmackey (string), a raw message to be encoded (string), and the current timestamp. It returns the encoded message ([]byte) using the group's HMAC key.  Note that the decrypt function will only work if the encrypted message is decrypted within 30 seconds of the set timestamp, otherwise the payload is expired and an error will be returned.
* DecodeHMAC- Takes a group's key (string) and the encoded message ([]byte).  It returns the decoded message (string).  Note that this decrypt function will only work if the encrypted message is decrypted within 30 seconds of the set timestamp, otherwise the payload is expired and an error will be returned.
