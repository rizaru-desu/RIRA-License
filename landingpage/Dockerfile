# Stage 1: Build static assets on the runner's native architecture ($BUILDPLATFORM)
FROM --platform=$BUILDPLATFORM oven/bun:1-alpine AS build

WORKDIR /app

# Copy package manifests and lockfile
COPY package.json bun.lock* ./

# Install dependencies with lockfile enforcement
RUN bun install --frozen-lockfile

# Copy landing page source and build
COPY . .
RUN bun run build

# Stage 2: Serve static files with Nginx on the target platform (e.g. linux/arm64)
FROM nginx:alpine AS runtime

# Replace default nginx configuration with custom SPA config
RUN rm -f /etc/nginx/conf.d/default.conf
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Copy Vite build output from build stage
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
