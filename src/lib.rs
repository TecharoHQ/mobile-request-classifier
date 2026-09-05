wit_bindgen::generate!({
    world: "phone-classifier",
});

pub struct PhoneClassifierWorld;
export!(PhoneClassifierWorld);

impl Guest for PhoneClassifierWorld {
    fn is_mobile(req: HttpRequest) -> Result<MobileLikelihood, String> {
        let is_slow = {
            let ua = req.user_agent.orig;
            match req {
                _ if req.proto.major == 1 => true,
                _ if ua.contains("Mobile/") && ua.contains("Version/1") => true,
                _ if ua.contains("CrOS") => true,
                _ if ua.contains("armv7l") => true,
                _ if ua.contains("i686") => true,
                _ => false,
            }
        };

        match req.sec_ch_ua {
            Some(sec_ch_ua) => {
                let mut result = MobileLikelihood {
                    is_phone: false,
                    is_slow: is_slow || sec_ch_ua.bitness == "32",
                    is_bot: req.user_agent.bot,
                };

                if let Some(mobile) = sec_ch_ua.mobile {
                    if mobile && req.user_agent.mobile {
                        result.is_phone = true;
                    }
                }

                Ok(result)
            }
            None => Ok(MobileLikelihood {
                is_slow,
                is_phone: req.user_agent.mobile,
                is_bot: req.user_agent.bot,
            }),
        }
    }
}
