"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.ScanType = exports.QueryOptimizer = exports.QueryType = exports.KNIRVQLParser = void 0;
var knirvql_1 = require("./knirvql");
Object.defineProperty(exports, "KNIRVQLParser", { enumerable: true, get: function () { return knirvql_1.KNIRVQLParser; } });
Object.defineProperty(exports, "QueryType", { enumerable: true, get: function () { return knirvql_1.QueryType; } });
var optimizer_1 = require("./optimizer");
Object.defineProperty(exports, "QueryOptimizer", { enumerable: true, get: function () { return optimizer_1.QueryOptimizer; } });
Object.defineProperty(exports, "ScanType", { enumerable: true, get: function () { return optimizer_1.ScanType; } });
//# sourceMappingURL=index.js.map