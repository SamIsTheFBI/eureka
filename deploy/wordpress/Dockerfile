FROM wordpress:6.7-apache

# Install WP-CLI, mariadb client, and unzip
RUN apt-get update && apt-get install -y --no-install-recommends \
    unzip \
    mariadb-client \
    curl \
    && rm -rf /var/lib/apt/lists/*

RUN curl -sSL https://raw.githubusercontent.com/wp-cli/builds/gh-pages/phar/wp-cli.phar -o /usr/local/bin/wp && \
    chmod +x /usr/local/bin/wp

# Pre-bake WooCommerce plugin into wordpress source
RUN curl -sSL https://downloads.wordpress.org/plugin/woocommerce.9.3.3.zip -o /tmp/wc.zip && \
    unzip -q /tmp/wc.zip -d /usr/src/wordpress/wp-content/plugins/ && \
    rm /tmp/wc.zip

# Configure Apache port 8080
RUN sed -i 's/Listen 80/Listen 8080/' /etc/apache2/ports.conf && \
    sed -i 's/<VirtualHost \*:80>/<VirtualHost \*:8080>/' /etc/apache2/sites-available/000-default.conf

EXPOSE 8080
