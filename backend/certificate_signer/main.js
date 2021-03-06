const { Kafka } = require('kafkajs');
const config = require('./config/config');
const signer = require('./signer');

console.log('Using ' + config.KAFKA_BOOTSTRAP_SERVER)
const kafka = new Kafka({
  clientId: 'divoc-cert',
  brokers: config.KAFKA_BOOTSTRAP_SERVER.split(",")
});

const consumer = kafka.consumer({ groupId: 'certificate_signer' });
const producer = kafka.producer({allowAutoTopicCreation: true});

const REGISTRY_SUCCESS_STATUS = "SUCCESSFUL";
const REGISTRY_FAILED_STATUS = "UNSUCCESSFUL";

(async function() {
  await consumer.connect();
  await producer.connect();
  await consumer.subscribe({topic: config.CERTIFY_TOPIC, fromBeginning: true});

  await consumer.run({
    eachMessage: async ({topic, partition, message}) => {
      console.log({
        value: message.value.toString(),
        uploadId: message.headers.uploadId ? message.headers.uploadId.toString():'',
        rowId: message.headers.rowId ? message.headers.rowId.toString():'',
      });
      let uploadId = message.headers.uploadId ? message.headers.uploadId.toString() : '';
      let rowId = message.headers.rowId ? message.headers.rowId.toString() : '';
      try {
        jsonMessage = JSON.parse(message.value.toString());
        await signer.signAndSave(jsonMessage)
          .then(res => {
            console.log(`statusCode: ${res.status}`);
            console.log(res);
            let errMsg;
            if (res.status === 200) {
              sendCertifyAck(res.data.params.status, uploadId, rowId, res.data.params.errmsg);
              producer.send({
                topic: config.CERTIFIED_TOPIC,
                messages: [{key: null, value: message.value.toString()}]
              });
            } else {
              errMsg = "error occurred while signing/saving of certificate - " + res.status;
              sendCertifyAck(REGISTRY_FAILED_STATUS, uploadId, rowId, errMsg)
            }
          })
          .catch(error => {
            console.error(error)
            sendCertifyAck(REGISTRY_FAILED_STATUS, uploadId, rowId, error.message)
          });
      } catch (e) {
        console.error("ERROR: " + e.message)
      }
    },
  })
})();

async function sendCertifyAck(status, uploadId, rowId, errMsg="") {
  if (config.ENABLE_CERTIFY_ACKNOWLEDGEMENT) {
    if (status === REGISTRY_SUCCESS_STATUS) {
      producer.send({
        topic: 'certify_ack',
        messages: [{
          key: null,
          value: JSON.stringify({
            uploadId: uploadId,
            rowId: rowId,
            status: 'SUCCESS',
            errorMsg: ''
          })}]})
    } else if (status === REGISTRY_FAILED_STATUS) {
      producer.send({
        topic: 'certify_ack',
        messages: [{
          key: null,
          value: JSON.stringify({
            uploadId: uploadId,
            rowId: rowId,
            status: 'FAILED',
            errorMsg: errMsg
          })}]})
    }
  }
}

