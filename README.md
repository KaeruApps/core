# Kaeru

The shared foundation for the self-hosted Kaeru Platform. Kaeru provides common
infrastructure and coordination while each platform service remains independently
developed and deployed.

## Development

Run the backend locally with authentication bypassed:

```sh
npm run dev:backend
```

This sets `KAERU_DEV_AUTH=true`, provides a runtime-only local user with the
Core `admin` role, and reports the application as initialized. Never enable
this setting on a server exposed to untrusted networks. Production startup via
`npm start` leaves the bypass disabled.

## License

Kaeru Core is licensed under the [GNU Affero General Public License v3.0](LICENSE).
