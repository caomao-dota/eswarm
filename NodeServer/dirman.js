/*nodejs递归创建目录，同步和异步方法。在官方API中只提供了最基本的方法，只能创建单级目录，如果要创建一个多级的目录（./aaa/bbb/ccc）就只能一级一级的创建，感觉不是很方便，因此简单写了两个支持多级目录创建的方法。 

*/
/** 
 * Created by RockeyCai on 16/2/22. 
 * 创建文件夹帮助类 
 */  
  
var fs = require("fs");  
var path = require("path");  
  
//递归创建目录 异步方法  
function mkdirs(dirname, callback) {  
    fs.exists(dirname, function (exists) {  
        if (exists) {  
            callback();  
        } else {  
            //console.log(path.dirname(dirname));  
            mkdirs(path.dirname(dirname), function () {  
                fs.mkdir(dirname, callback);  
            });  
        }  
    });  
}  
  
//递归创建目录 同步方法  
function mkdirsSync(dirname) {  
    //console.log(dirname);  
    if (fs.existsSync(dirname)) {  
        return true;  
    } else {  
        if (mkdirsSync(path.dirname(dirname))) {  
            fs.mkdirSync(dirname);  
            return true;  
        }  
    }  
}  
  
module.exports.mkdirs = mkdirs;  
  
module.exports.mkdirsSync= mkdirsSync;  
  
//调用  
//mkdirsSync("./aa/bb/cc" , null);  
//mkdirs("./aa/bb/cc", function (ee) {  
//    console.log(ee)  
//}); 