CREATE DATABASE social;
USE social;


# Dump of table Friend
# ------------------------------------------------------------

DROP TABLE IF EXISTS `Friend`;

CREATE TABLE `Friend` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint(20) unsigned DEFAULT NULL,
  `friend_user_id` bigint(20) unsigned DEFAULT NULL,
  `request_accepted` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `user_id_index` (`user_id`,`friend_user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;



# Dump of table Messages
# ------------------------------------------------------------

DROP TABLE IF EXISTS `Messages`;

CREATE TABLE `Messages` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint(20) unsigned DEFAULT NULL,
  `user_id_to` bigint(20) unsigned DEFAULT NULL,
  `msg_type` enum('In','Out') NOT NULL DEFAULT 'In',
  `message` longtext NOT NULL,
  `ts` bigint(20) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `user_id` (`user_id`,`user_id_to`,`ts`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;



# Dump of table Timeline
# ------------------------------------------------------------

DROP TABLE IF EXISTS `Timeline`;

CREATE TABLE `Timeline` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint(20) unsigned DEFAULT NULL,
  `source_user_id` bigint(20) unsigned DEFAULT NULL,
  `message` longtext NOT NULL,
  `ts` bigint(20) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `user_id` (`user_id`,`ts`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;



# Dump of table User
# ------------------------------------------------------------

DROP TABLE IF EXISTS `User`;

CREATE TABLE `User` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `email` varchar(255) NOT NULL DEFAULT '',
  `password` varchar(80) NOT NULL DEFAULT '',
  `name` varchar(255) NOT NULL DEFAULT '',
  PRIMARY KEY (`id`),
  UNIQUE KEY `email_index` (`email`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
