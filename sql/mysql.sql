-- Create syntax for TABLE 'addresses'
CREATE TABLE `addresses` (
  `address` varchar(52) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '',
  `is_node` tinyint(1) unsigned NOT NULL DEFAULT '0',
  `amount` bigint(30) unsigned DEFAULT '0',
  `frozen` bigint(30) unsigned DEFAULT '0',
  `forging` bigint(30) unsigned DEFAULT '0',
  `delegated` bigint(30) unsigned DEFAULT '0',
  `undelegated` bigint(30) unsigned DEFAULT '0',
  `delegated_amount` bigint(30) unsigned DEFAULT '0',
  `tx_count` bigint(30) unsigned DEFAULT '0',
  `tx_in` bigint(30) unsigned DEFAULT '0',
  `tx_out` bigint(30) unsigned DEFAULT '0',
  `block_number` bigint(30) unsigned DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`address`),
  KEY `amount` (`amount`),
  KEY `frozen` (`frozen`),
  KEY `forging` (`forging`),
  KEY `tx_count` (`tx_count`),
  KEY `tx_in` (`tx_in`),
  KEY `updated_at` (`updated_at`),
  KEY `delegated_amount` (`delegated_amount`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- Create syntax for TABLE 'blocks'
CREATE TABLE `blocks` (
  `number` int(11) unsigned NOT NULL,
  `hash` varchar(65) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL,
  `prev_hash` varchar(65) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL,
  `tx_hash` varchar(65) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL,
  `timestamp` timestamp NULL DEFAULT NULL,
  `count_txs` int(11) DEFAULT NULL,
  `sign` varchar(65) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL,
  `size` bigint(20) DEFAULT NULL,
  `type` varchar(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`number`),
  UNIQUE KEY `hash` (`hash`),
  KEY `timestamp` (`timestamp`),
  KEY `count_txs` (`count_txs`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- Create syntax for TABLE 'nodes'
CREATE TABLE `nodes` (
  `address` varchar(52) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '',
  `node_type` varchar(25) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT 'proxy',
  `name` varchar(512) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL,
  `ip` varchar(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT '1',
  `last_updated` datetime DEFAULT NULL,
  `last_checked` datetime DEFAULT NULL,
  `latitude` decimal(11,6) DEFAULT '0.000000',
  `longitude` decimal(11,6) DEFAULT '0.000000',
  `country_short` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT '',
  `country_long` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT '',
  `region` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT '',
  `city` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT '',
  `mg_trust` varchar(5) COLLATE utf8mb4_general_ci DEFAULT NULL,
  `mg_geo` varchar(5) COLLATE utf8mb4_general_ci DEFAULT NULL,
  `mg_status` tinyint(1) DEFAULT NULL,
  `mg_roi` varchar(11) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL,
  `mg_qps` varchar(11) COLLATE utf8mb4_general_ci DEFAULT NULL,
  `mg_rps` varchar(11) COLLATE utf8mb4_general_ci DEFAULT NULL,
  `is_online` tinyint(1) unsigned zerofill NOT NULL DEFAULT '0',
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`address`),
  KEY `country_short` (`country_short`),
  KEY `mg_trust` (`mg_trust`,`mg_roi`,`mg_status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;