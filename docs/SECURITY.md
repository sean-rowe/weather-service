# Security Configuration

## Environment Variables

This application uses environment variables for configuration to avoid hardcoded secrets in the codebase.

### Required Environment Variables

Before running the application, you must set the following environment variables:

```bash
# Database password (REQUIRED)
export DB_PASSWORD="your_secure_database_password"

# Grafana admin password (REQUIRED for docker-compose)
export GF_SECURITY_ADMIN_PASSWORD="your_secure_grafana_password"
```

### Setting Up Environment

1. **Copy the example environment file:**
   ```bash
   cp .env.example .env
   ```

2. **Edit `.env` with your secure values:**
   ```bash
   # Update these with secure values
   DB_PASSWORD=your_secure_password_here
   GF_SECURITY_ADMIN_PASSWORD=your_secure_grafana_password_here
   ```

3. **Load environment variables:**
   ```bash
   # For bash/zsh
   source .env
   
   # Or use docker-compose with env file
   docker-compose --env-file .env up
   ```

### Security Best Practices

- **Never commit `.env` files** - They are already in `.gitignore`
- **Use strong passwords** - Minimum 16 characters with mixed case, numbers, and symbols
- **Rotate secrets regularly** - Change passwords and keys periodically
- **Use secret management** - For production, consider using:
  - HashiCorp Vault
  - AWS Secrets Manager
  - Azure Key Vault
  - Kubernetes Secrets
- **Limit database permissions** - Use principle of least privilege
- **Enable SSL/TLS** - Set `DB_SSLMODE=require` for production databases

### Docker Compose Security

The docker-compose.yml file uses Docker's variable substitution with required checks:

- `${VAR:?message}` - Fails if variable is not set
- `${VAR:-default}` - Uses default if variable is not set

This ensures critical secrets like passwords cannot be accidentally left empty.

### Production Deployment

For production deployments:

1. Use external secret management systems
2. Never use default passwords
3. Enable database SSL
4. Use non-root database users with minimal privileges
5. Regularly audit and rotate credentials
6. Monitor for suspicious activity