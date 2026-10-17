CREATE DATABASE IF NOT EXISTS `identity` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS 'identity'@'%' IDENTIFIED BY 'identity';
GRANT ALL PRIVILEGES ON `identity`.* TO 'identity'@'%';
