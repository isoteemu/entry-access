# Door Entry System

## Premise

1. System generates a unique QR code for each door entry request.
2. QR codes are rotated regularly to enhance security (env:`TOKEN_EXPIRY` x 0.5).
3. QR code contains a JWT with entry ID and nonce.
4. QR code is scanned at the door for entry.
5. System validates the JWT and nonce before granting access.

## Settings

- `SECRET`: (Random) Secret key for signing JWTs. **Must** be set for production.
- `TOKEN_EXPIRY`: JWT expiry time in seconds. Default is 30 seconds. QR code is rotated every half of this.
- `LOG_LEVEL`: Logging level. Options are `debug`, `info`, `warn`, `error`. Default is `info`.
- `NONCE_STORE`: Type of nonce store. Options are `memory` (default) or `redis`.
