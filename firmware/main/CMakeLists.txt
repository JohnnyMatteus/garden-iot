idf_component_register(
    SRCS "sensor_node.c" "sensors.c" "net.c" "mqtt_link.c" "cert_store.c" "node_state.c"
         "flash_storage.c" "power.c"
    INCLUDE_DIRS "."
    REQUIRES garden_core
    PRIV_REQUIRES esp_wifi esp_event esp_netif nvs_flash esp_adc driver mqtt esp_partition
                  esp_timer
)
