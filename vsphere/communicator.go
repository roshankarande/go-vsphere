package vsphere

func Connect() error {

	return nil
}

//// Communicator is an interface that must be implemented by all communicators
//// used for any of the provisioners
//type Communicator interface {
//	// Connect is used to setup the connection
//	Connect(terraform.UIOutput) error
//
//	// Disconnect is used to terminate the connection
//	Disconnect() error
//
//	// Timeout returns the configured connection timeout
//	Timeout() time.Duration
//
//	// ScriptPath returns the configured script path
//	ScriptPath() string
//
//	// Start executes a remote command in a new session
//	Start(*remote.Cmd) error
//
//	// Upload is used to upload a single file
//	Upload(string, io.Reader) error
//
//	// UploadScript is used to upload a file as an executable script
//	UploadScript(string, io.Reader) error
//
//	// UploadDir is used to upload a directory
//	UploadDir(string, string) error
//}