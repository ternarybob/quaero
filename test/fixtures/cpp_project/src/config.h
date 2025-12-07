#ifndef CONFIG_H
#define CONFIG_H

#define APP_NAME "MyApp"
#define APP_VERSION "1.0.0"

// Configuration sources
#define CONFIG_FROM_ENV 1
#define CONFIG_FROM_FILE 1
#define CONFIG_FROM_REGISTRY 0  // Windows only

// Database settings (hardcoded defaults)
#define DB_HOST "localhost"
#define DB_PORT 5432

// External service dependencies
#define REDIS_ENABLED 1
#define ELASTICSEARCH_ENABLED 0

#endif // CONFIG_H
