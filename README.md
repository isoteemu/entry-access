# Door Entry System

## Premise

1. System generates a unique QR code for each door entry request.
2. QR codes are rotated regularly to enhance security (env:`TOKEN_EXPIRY` x 0.5).
3. QR code contains a JWT with entry ID and nonce.
4. QR code is scanned at the door for entry.
5. System validates the JWT and nonce before granting access.

### Technologies used

- Go (Gin framework)
- slog (logging)
- JWT (github.com/golang-jwt/jwt/v5)
- QR Code generation (github.com/skip2/go-qrcode)
- Tailwind CSS (frontend)

## Access lists

Access lists are stored as CSV files in the folder defined by `ACCESS_LIST_FOLDER` (default `instance/`).

Format follows Sisu export format, meaning file **needs** to be UTF-16 with LE.

## TODO

- [ ] Ingress setup for deployment
  - [ ] Rate limiting

- [ ] Storage driver (Redis, database, etc.)
    - [ ] Move nonce store to persistent storage 
    - [ ] Device provisioning data

- [ ] Admin interface
    - [ ] Upload access lists
        - [ ] Define starting and ending dates
    - [ ] Show loaded access lists

- [ ] PII handling (GDPR compliance)
    - [ ] Anonymize logs after a set time period
        > "[--] no longer than is necessary for the purposes for which the personal data are processed."
    - [ ] Data retention policy
        - [ ] Security logs
        - [ ] Operational logs
    - [ ] Provide data export for users (csv)

- [ ] Show success page on entry device

- [ ] Version check for entry device

### Errors

Error handlers to app:

 - [x] Network errors
 - [ ] QR Code loading errors
 - [ ] Backend health errors
 - [ ] Expired provisioning session
 - [ ] Tampering detected


### User authentication

Alternatives:
 - Using LDAP? 
 - Using a single email link
    - [ ] Mark authentication to be consumed, or not.
 - MyJYU QR code login

### Provisioning

- Enter the provisioning page
- Random device_id is generated, and stored on device.
- QR Code is displayed, pointing to the provisioning URL. Url contains client_id and client IP.
- When accessed on another device, authorization is performed
    - If authorized to manage devices, user is shown a selection of entryways.
    - User selects the entryway to authorize the device to.
    - If successful, device is added on provisioned devices list.
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

- `ALLOWED_NETWORKS`: Comma-separated list of CIDR ranges that are allowed to access the API. Example: `192.168.1.0/24,192.168.2.1/32`
- `ACCESS_LIST_FOLDER`: Folder path where CSV access lists are stored. Default is `instance/`.

- `TOKEN_EXPIRY`: JWT expiry time in seconds. Default is 60 seconds. QR code is `QR_EXPIRY_SKEW` seconds before this
- `NONCE_STORE`: Type of nonce store. Options are `memory` (default) or ... .
- `LOG_LEVEL`: Logging level. Options are `debug`, `info`, `warn`, `error`. Default is `info`.
- `GIN_MODE`: Gin framework mode. Options are `debug`, `release`, or `test`. Default is `debug`.

## Error codes
