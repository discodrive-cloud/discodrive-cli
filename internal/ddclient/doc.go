// Package ddclient is the shared REST client for the DiscoDrive server: device-token
// authentication (exchanged for a JWT that is cached and retried on 401), pairing
// (device-code flow), file operations (listing, path→id resolution, download/upload,
// mkdir/rename/move/delete, search), the user profile, and shares. Both dd-cli and
// dd-mcp are built on it — one implementation of the logic, two faces.
//
// Config and tokens are stored in ~/.config/discodrive/cli.json (see config.go).
package ddclient
