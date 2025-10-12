FROM swaggerapi/swagger-ui:latest

# Copy the combined OpenAPI spec
COPY combined.yaml /usr/share/nginx/html/combined.yaml

# Set environment variables
ENV SWAGGER_JSON_URL=/combined.yaml
ENV SWAGGER_UI_DEEP_LINKING=true
ENV SWAGGER_UI_DISPLAY_REQUEST_DURATION=true
ENV SWAGGER_UI_TRY_IT_OUT_ENABLED=true

# Expose port
EXPOSE 8080

# The base image already has the correct CMD
