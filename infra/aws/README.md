# EC2 + Nginx Deployment

This folder contains a simple EC2 deployment path for:

- Nginx reverse proxy (`:80`, `:443`)
- Go backend (private container network)
- Postgres (private container network)

## Important Architecture Rule

Swift/iOS should call the backend API only.
Do not connect mobile clients directly to Postgres.

## 1) Create EC2 Instance

Use Ubuntu 22.04+ and open inbound security-group rules:

- `80/tcp` from internet
- `443/tcp` from internet
- `22/tcp` from your IP only

Do not expose Postgres (`5432`) publicly.

## 2) Bootstrap EC2

Copy repo to the instance, then run:

```bash
cd queue_up/infra/aws
chmod +x bootstrap-ubuntu.sh deploy-ec2.sh
./bootstrap-ubuntu.sh
```

Log out/in (or reconnect SSH) so docker group membership applies.

## 3) Configure Secrets

```bash
cd queue_up/infra/aws
cp .env.example .env
```

Edit `.env` and set a strong `POSTGRES_PASSWORD`.

## 4) Deploy Stack

```bash
cd queue_up/infra/aws
./deploy-ec2.sh
```

## 5) Verify

```bash
curl http://localhost/health
curl http://<EC2_PUBLIC_IP>/health
```

## Optional TLS

- Place cert/key in `infra/aws/nginx/certs`
- Update `nginx/queue-up.conf` with `listen 443 ssl;` and cert paths
- Reload: `docker compose --env-file .env -f docker-compose.ec2.yml up -d`
