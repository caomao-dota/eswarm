

const rlp  = require("rlp")
const secp256k1 = require("secp256k1")
const ejsutil = require("ethereumjs-util")

const fs = require('fs')

/**
 * 
 * @param {数据} nodeId 
 * @param {被编码的收据} coded_receipts 
 */
function parseReceiptsV1(nodeId,coded_receipts){
 /*  pk_compact = Buffer.concat([Buffer([0x02]),in_result[1]])
                    pkOK = secp256k1.publicKeyVerify(pk_compact,true)
                    pbKey = secp256k1.publicKeyConvert(pk_compact,false)*/
                    receipts  = {
                        "version":1,
                        "account":nodeId
                    }
                    receiptsCount = coded_receipts && coded_receipts.length
                    for( j = 0; j < receiptsCount; j++) {
                        receipt_item = coded_receipts[j]
                        if(receipt_item && receipt_item.length == 3){
                            receipt = {
                               "stime":receipt_item[0],
                               "amount":receipt_item[1],
                               "signature":receipt_item[2]
                            }   
                            data =Buffer.concat([receipts.account,receipt.stime,receipt.amount]);
                            hash = ejsutil.rlphash(data)

                           
                            pubKey = secp256k1.recover(hash,receipt.signature.slice(0,64),1,true)
                            pubKey_uc = secp256k1.recover(hash,receipt.signature.slice(0,64),1,false)
                            signOk = secp256k1.verify(hash,receipt.signature.slice(0,64),pubKey)
                            if(signOk){
                                receipt.signer = ejsutil.pubToAddress(pubKey_uc,true)
                                if(receipts.items == undefined){
                                    receipts.items = [receipt] 
                                }else{
                                    receipts.items.push(receipt)
                                }
                            }else {
                                console.error("verify signature failed")
                            }
                        }
                    
                }
                return receipts
}
//从节点上传的收据集，按照如下的格式：
/*{
    version,DataProviderId,[{Stime,amount,signature},{Stime,amount,signature},....]
}
    在每一个签名里，可以恢复出签名者的公钥，通过公钥产生支付者的帐户地址

    生成一个收据集：{
        .version   版本号，目前为1
        .account   数据服务者（收据的接收者）帐户，这是一个32字节的帐户信息，最后20个字节是以太坊帐户
        .items     该帐户收到的数据集，是一个数组
        [
            {
                .stime  该收据的时间戳，是一个int64的数据，应用时，需要转化为以nanosecond为单位的时间
                .amount 该收据的总共收到的chunk的数量,目前是一个Buffer,应用时，需要转化成一个32位的整数
                .signature  一个65字节的Buffer，是通过sign(hash(account,stime,amount))的值
                .signer  该收据的签名者，是从signature中恢复出来的
            }
        ]
        .hash  以上所有的哈希值，用于验证是否提交过
    }

    应用层，通过parseReportReceipts处理后，产生一个如上述所记录的收据，应用根据这个收据作如下的计费：
    我们设置每个chunk的手续费为1Gas
    1.根据将每个收据中的帐户以及amount，将相应的手续费转移到account中
    2.从account中扣除gas*10个手续费
    3.每一条收据，从account中扣除5个手续费
    4.2/3扣除的手续费，放到系统的专用帐户中去

    备注：***需要检查系统中有没有同一个account的来自于同一个signer的相同STIME的记录，有的话，按照原有的和新的最大值计算account的收益，即：
    1.如果原有数据库中的amount大于此amount，则不作处理
    2.如果新的amount更大，那么手续费是新的amount-原有的amount
    无论哪种情况，都要从account帐户中扣除5GAS的手续费

*/
function parseReportReceipts(data){
    in_result = rlp.decode(data)
    //result 0 is incoming node store, and result[1] is receipts
     console.log(JSON.stringify(in_result))

    if(in_result && in_result.length == 3 ){  //it is a receipts
            if( in_result[0] && in_result[0].length == 1 &&  in_result[0][0] == 0x01){
                 return parseReceiptsV1(in_result[1],in_result[2])       
            }
    }          
}


function VerifyReport(data){
    if(data.length < 65){
        return undefined,"insufficent ata"
    }else {
        signature = data.slice(0,64)
        recovery = data.slice(64,65)[0]
        result   = data.slice(65,data.length)
        hash = ejsutil.rlphash(result)


        apubKey = secp256k1.recover(hash,signature,recovery,true)

        apubKey_uc = secp256k1.recover(hash,signature,recovery,false)
        signOk = secp256k1.verify(hash,signature,apubKey)

        if(signOk){
            receipt = {
                receipts:parseReportReceipts(result),
                hash:hash,
                signer:ejsutil.pubToAddress(apubKey,true)
            }
            return receipt
        }
    }
}

var formidable = require('formidable'),
    http = require('http'),
    util = require('util');
 
http.createServer(function(req, res) {
  if (req.url == '/receipts' && req.method.toLowerCase() == 'post') {

    var body = Buffer.alloc(0);   
    req.on('data', function(chunk){
      body = Buffer.concat([body,chunk]);
    }).on('end',function(){
       result =  VerifyReport(body)
    })
  }
 
  // show a file upload form
  res.writeHead(200, {'content-type': 'text/html'});
  res.end();
}).listen(8088);

const { randomBytes } = require('crypto')
function testSigner (){
    // or require('secp256k1/elliptic')
    //   if you want to use pure js implementation in node

    // generate message to sign
    const msg = randomBytes(32)

    // generate privKey
    let privKey
    do {
    privKey = randomBytes(32)
    } while (!secp256k1.privateKeyVerify(privKey))

    // get the public key in a compressed format
    const pubKey = secp256k1.publicKeyCreate(privKey)

    // sign the message
    const sigObj = secp256k1.sign(msg, privKey)
    pKey = secp256k1.recover(msg,sigObj.signature,sigObj.recovery,false)
    msg[0] += 1
    // verify the signature
    console.log(secp256k1.verify(msg, sigObj.signature, pKey))
    // => true
}
