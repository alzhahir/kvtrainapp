FROM node:20-alpine
WORKDIR /app
COPY frontend/package.json ./
RUN npm install --production=false
COPY frontend/ .
RUN npm run build
EXPOSE 3000
CMD ["npm", "start"]
