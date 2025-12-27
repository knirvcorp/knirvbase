use knirvbase::*;
use std::collections::HashMap;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    println!("KNIRVBASE Distributed Database Starting...");

    // Get app data directory
    let app_data_dir = std::env::var("XDG_DATA_HOME")
        .unwrap_or_else(|_| {
            let home = std::env::var("HOME").unwrap();
            format!("{}/.local/share/knirvbase", home)
        });

    std::fs::create_dir_all(&app_data_dir)?;

    // Create database
    let opts = Options {
        data_dir: app_data_dir,
        distributed_enabled: true,
        distributed_network_id: "".to_string(),
        distributed_bootstrap_peers: vec![],
    };

    let db = DB::new(opts).await?;

    // Create network
    let network_id = db.create_network(NetworkConfig {
        network_id: "consortium-1".to_string(),
        name: "Consortium 1".to_string(),
        collections: HashMap::new(),
        bootstrap_peers: vec![],
        default_posting_network: "".to_string(),
        auto_post_classifications: vec![],
        private_by_default: true,
        encryption: Default::default(),
        replication: Default::default(),
        discovery: Default::default(),
    }).await?;

    // Create collections
    let auth_coll = db.collection("auth").await?;
    let memory_coll = db.collection("memory").await?;

    // Attach to network
    auth_coll.attach_to_network(&network_id).await?;
    memory_coll.attach_to_network(&network_id).await?;

    // Example usage
    println!("KNIRVBASE Distributed Database Started");

    // Example KNIRVQL
    let parser = KNIRVQLParser::new();

    // Insert memory entry
    let mut doc = HashMap::new();
    doc.insert("id".to_string(), serde_json::Value::String("mem1".to_string()));
    doc.insert("entryType".to_string(), serde_json::Value::String("MEMORY".to_string()));
    let mut payload = HashMap::new();
    payload.insert("source".to_string(), serde_json::Value::String("web-scrape".to_string()));
    payload.insert("data".to_string(), serde_json::Value::String("some data".to_string()));
    payload.insert("vector".to_string(), serde_json::to_value(vec![0.45f64, 0.12f64])?);
    doc.insert("payload".to_string(), serde_json::to_value(payload)?);

    memory_coll.insert(doc).await?;

    println!("Inserted memory entry");

    // Query memory
    let query = parser.parse("GET MEMORY WHERE source = \"web-scrape\"")?;
    let results = query.execute(&db.db, &*memory_coll.c.read().await).await?;
    println!("Memory results: {:?}", results);

    println!("KNIRVBASE running. Press Ctrl+C to exit.");

    // Keep running
    tokio::signal::ctrl_c().await?;
    db.shutdown().await?;

    Ok(())
}