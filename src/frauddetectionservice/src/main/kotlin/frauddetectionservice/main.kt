
package frauddetectionservice

import io.opentelemetry.api.GlobalOpenTelemetry
import io.opentelemetry.api.common.AttributeKey.stringKey
import io.opentelemetry.api.common.Attributes
import io.opentelemetry.api.metrics.LongCounter
import io.opentelemetry.api.trace.Span
import io.opentelemetry.instrumentation.annotations.WithSpan
import org.apache.kafka.clients.consumer.ConsumerConfig.*
import org.apache.kafka.clients.consumer.KafkaConsumer
import org.apache.kafka.common.serialization.ByteArrayDeserializer
import org.apache.kafka.common.serialization.StringDeserializer
import oteldemo.Demo.*
import java.time.Duration.ofMillis
import java.util.*
import kotlin.system.exitProcess


const val topic = "orders"
const val groupID = "frauddetectionservice"
val actionAttributes: Attributes = Attributes.of(
    stringKey("sinktype"), "SinkType.SQL_EXECUTE",
    stringKey("rpc.service"), "otel.fraudDetection",
    stringKey("rpc.method"), "TrackUserAction")

fun main(args: Array<String>) {
    val props = Properties()
    props[KEY_DESERIALIZER_CLASS_CONFIG] = StringDeserializer::class.java.name
    props[VALUE_DESERIALIZER_CLASS_CONFIG] = ByteArrayDeserializer::class.java.name
    props[GROUP_ID_CONFIG] = groupID
    val bootstrapServers = System.getenv("KAFKA_SERVICE_ADDR")
    if (bootstrapServers == null) {
        println("KAFKA_SERVICE_ADDR is not supplied")
        exitProcess(1)
    }
    props[BOOTSTRAP_SERVERS_CONFIG] = bootstrapServers
    val consumer = KafkaConsumer<String, ByteArray>(props).apply {
        subscribe(listOf(topic))
    }

    var totalCount = 0L
    val meter = GlobalOpenTelemetry.getMeter("contrast-actions")
    val actionCounter: LongCounter = meter
        .counterBuilder("rpc.server.sink")
        .setDescription("Counts sinktypes seen in a route")
        .build()

    consumer.use {
        while (true) {
            totalCount = consumer
                .poll(ofMillis(100))
                .fold(totalCount) { accumulator, record ->
                    val newCount = accumulator + 1
                    val orders = OrderResult.parseFrom(record.value())
                    println("Consumed record with orderId: ${orders.orderId}, and updated total count to: $newCount")
                    fakeDbUpdate1(actionCounter)
                    fakeDbUpdate2(actionCounter)
                    newCount
                }
        }
    }
}

@WithSpan
fun fakeDbUpdate1(cnt: LongCounter) {
    var span = Span.current()
    span.setAttribute("db.system", "mysql")
    span.setAttribute("db.name", "fraud")
    span.setAttribute("db.sql.table", "actions")
    span.setAttribute("db.operation", "UPSERT")
    span.setAttribute("net.peer.name", "silo1.example.com")

    cnt.add(1, actionAttributes)
}

@WithSpan
fun fakeDbUpdate2(cnt: LongCounter) {
    var span = Span.current()
    span.setAttribute("db.system", "mysql")
    span.setAttribute("db.name", "datalake")
    span.setAttribute("db.sql.table", "user_behavior")
    span.setAttribute("db.operation", "UPSERT")
    span.setAttribute("net.peer.name", "datalake.example.com")
    cnt.add(1, actionAttributes)
}