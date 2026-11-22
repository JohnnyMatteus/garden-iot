#pragma once

#include "sdkconfig.h"

#define TOPIC_BASE "horta/" CONFIG_GARDEN_TENANT "/" CONFIG_GARDEN_HORTA "/" CONFIG_GARDEN_NODE_ID
#define TOPIC_TELEMETRY TOPIC_BASE "/telemetria"
#define TOPIC_LWT TOPIC_BASE "/lwt"
