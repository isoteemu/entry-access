# Door Entry System

## Premise

1. System generates a unique QR code for each door entry request.
2. QR codes are rotated regularly to enhance security (env:`TOKEN_EXPIRY` x 0.5).
3. QR code contains a JWT with entry ID and nonce.
4. QR code is scanned at the door for entry.
5. System validates the JWT and nonce before granting access.

## TODO

### User authentication

Alternatives:
 - Using LDAP?
 - Using a single email link

### Provisioning

- Enter the provisioning page
- Random device_id is generated, and stored on localstorage
- QR Code is displayed, pointing to the provisioning URL. Url contains client_id and client IP.
- When accessed on another device, authorization is performed
    - If successfull, device is added on allowed list.
- Device pulls for being authorized. If authorized, server returns JWT with:
    - expiration (2 Days)
    - device_id
    - Client IP
    - allowed entryway

- Device automatically checks that:
    - If there is less than 1 day until expiration, refresh is performed.

### User list

- Sisu

## Settings

- `SECRET`: (Random) Secret key for signing JWTs. **Must** be set for production.
- `TOKEN_EXPIRY`: JWT expiry time in seconds. Default is 60 seconds. QR code is `QR_EXPIRY_SKEW` seconds before this
- `ALLOWED_NETWORKS`: Comma-separated list of CIDR ranges that are allowed to access the API. Example: `192.168.1.0/24,192.168.2.1/32`
- `NONCE_STORE`: Type of nonce store. Options are `memory` (default) or ... .
- `LOG_LEVEL`: Logging level. Options are `debug`, `info`, `warn`, `error`. Default is `info`.
- `GIN_MODE`: Gin framework mode. Options are `debug`, `release`, or `test`. Default is `debug`.
