extern crate rsa;

use rand::rngs::OsRng;
use rsa::{PaddingScheme, PublicKey, RSAPrivateKey, RSAPublicKey};
use wasm_bindgen::prelude::*;

#[wasm_bindgen]
pub enum Outcome {
    Success,
    Failure,
}

#[wasm_bindgen]
pub struct RSAEncryptionManager {
    priv_key: Option<RSAPrivateKey>,
    pub_key: Option<RSAPublicKey>,
}

#[wasm_bindgen]
impl RSAEncryptionManager {
    pub fn init() -> Self {
        RSAEncryptionManager {
            priv_key: None,
            pub_key: None,
        }
    }

    fn rsa_key_parser() -> Box<dyn Fn(&str) -> String> {
        Box::new(|rsa_key: &str| -> String {
            rsa_key
                .lines()
                .filter(|line| {
                    !line.starts_with("-")
                        && !line.starts_with("Proc-Type:")
                        && !line.starts_with("DEK-Info:")
                })
                .fold(String::new(), |mut data, line| {
                    data.push_str(&line);
                    data
                })
        })
    }

    pub fn load_pub_key(&mut self, pub_key: &str) -> Outcome {
        let der_encoded = Self::rsa_key_parser()(pub_key);
        let der_bytes = base64::decode(der_encoded).unwrap();
        let rsa_public_key = RSAPublicKey::from_pkcs1(&der_bytes).unwrap();
        self.pub_key = Some(rsa_public_key);
        Outcome::Success
    }

    pub fn load_priv_key(&mut self, priv_key: &str) -> Outcome {
        let der_encoded = Self::rsa_key_parser()(priv_key);
        let der_bytes = base64::decode(der_encoded).unwrap();
        let rsa_private_key = RSAPrivateKey::from_pkcs1(&der_bytes).unwrap();
        let rsa_public_key = rsa_private_key.to_public_key();
        self.priv_key = Some(rsa_private_key);
        self.pub_key = Some(rsa_public_key);
        Outcome::Success
    }

    pub fn encrypt(&self, msg: &[u8]) -> Option<Vec<u8>> {
        let mut rng = OsRng;
        let padding = PaddingScheme::new_pkcs1v15_encrypt();
        match &self.pub_key {
            Some(public_key) => {
                let encrypted_msg = public_key.encrypt(&mut rng, padding, msg).unwrap();
                Some(encrypted_msg)
            }
            _ => None,
        }
    }

    pub fn decrypt(&self, msg: &[u8]) -> Option<Vec<u8>> {
        let padding = PaddingScheme::new_pkcs1v15_encrypt();
        match &self.priv_key {
            Some(private_key) => {
                let decrypted_msg = private_key.decrypt(padding, msg).unwrap();
                Some(decrypted_msg)
            }
            _ => None,
        }
    }
}
