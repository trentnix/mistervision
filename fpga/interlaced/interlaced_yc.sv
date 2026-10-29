// SPDX-License-Identifier: GPL-2.0-or-later
// Y/C conversion for the standalone Menu core's interlaced scaler raster.
// Main supplies encoder parameters for Menu's 20 MHz native video clock.
// The standalone raster uses 25.175 MHz / 2, so phase increments multiply by
// 1600/1007 and burst durations multiply by 1007/1600. Keep Main's selected
// color standard and carrier adjustment, including PAL60 and PAL-M.
// This module must only be selected for the standalone ilace_auto modeline.
module interlaced_yc (
    input clk,
    input [39:0] phase_inc,
    input [16:0] burst_range,
    input pal, cvbs,
    input [23:0] rgb,
    input hs, vs, cs, de,
    output [23:0] video,
    output hs_out, vs_out, cs_out, de_out
);
    // Configuration changes are infrequent. Synchronize the stable settings
    // into the scaler clock domain before computing its encoder parameters.
    (* async_reg = "true" *) reg [58:0] config_meta = 0;
    (* async_reg = "true" *) reg [58:0] config_sync = 0;
    reg [50:0] phase_product = 0;
    reg [44:0] phase_times24 = 0;
    reg [40:0] phase_plus7 = 0;
    reg [58:0] config_product = 0;
    reg [26:0] burst_start_product = 0, burst_end_product = 0;
    reg [58:0] config_calc = 0, converting = 0, active = 0;
    reg [5:0] remaining = 0;
    reg busy = 0;
    reg [39:0] scaled_phase = 0;
    reg [16:0] scaled_burst = 0;
    reg start = 0;
    wire advance = busy && remaining != 0;
    wire [50:0] phase_quotient, start_quotient, end_quotient;

    // Serial constant division avoids a long combinational path in the HDMI
    // clock domain. Settings take 52 cycles to publish as one coherent set.
    interlaced_yc_divider #(1007) phase_divider (
        clk, start, advance, phase_product, phase_quotient);
    interlaced_yc_divider #(1600) start_divider (
        clk, start, advance, {24'b0, burst_start_product}, start_quotient);
    interlaced_yc_divider #(1600) end_divider (
        clk, start, advance, {24'b0, burst_end_product}, end_quotient);

    always @(posedge clk) begin
        start <= !busy && !start && (config_calc != active);
        config_meta <= {pal, cvbs, burst_range, phase_inc};
        config_sync <= config_meta;
        config_product <= config_sync;
        config_calc <= config_product;
        // x*1600+503 = (x*24 + x+7)*64+55. Split the additions
        // across registers so configuration also meets the HDMI clock timing.
        phase_times24 <= {1'b0, config_sync[39:0], 4'b0} + {2'b0, config_sync[39:0], 3'b0};
        phase_plus7 <= {1'b0, config_sync[39:0]} + 41'd7;
        phase_product <= {45'(phase_times24 + phase_plus7), 6'd55};
        burst_start_product <= {20'b0, config_product[56:50]} * 27'd1007;
        burst_end_product <= {17'b0, config_product[49:40]} * 27'd1007;
        if (start) begin
            converting <= config_calc;
            remaining <= 51;
            busy <= 1;
        end else if (advance) begin
            remaining <= remaining - 1'b1;
        end else if (busy) begin
            // Main writes the settings in several SPI words. Discard a result
            // if those settings changed during conversion, then retry.
            if (converting == config_calc) begin
                scaled_phase <= phase_quotient[39:0];
                scaled_burst <= {start_quotient[6:0], end_quotient[9:0]};
                active <= converting;
            end
            busy <= 0;
        end
    end

    yc_out encoder (
        .clk(clk), .PHASE_INC(scaled_phase), .PAL_EN(active[58]),
        .CVBS(active[57]), .COLORBURST_RANGE(scaled_burst),
        .din(rgb), .hsync(hs), .vsync(vs), .csync(cs), .de(de),
        .dout(video), .hsync_o(hs_out), .vsync_o(vs_out),
        .csync_o(cs_out), .de_o(de_out)
    );
endmodule

// One quotient bit per clock. Both divisors fit eleven bits. The caller loads
// a numerator, advances exactly 51 times, then reads the complete quotient.
module interlaced_yc_divider #(parameter [11:0] DIVISOR = 12'd1007) (
    input clk, load, advance,
    input [50:0] numerator,
    output reg [50:0] quotient = 0
);
    reg [50:0] bits_left = 0;
    reg [10:0] remainder = 0;
    wire [11:0] trial = {remainder, bits_left[50]};
    wire subtract = trial >= DIVISOR;
    wire [11:0] reduced = subtract ? trial - DIVISOR : trial;
    always @(posedge clk) begin
        if (load) begin
            bits_left <= numerator;
            remainder <= 0;
            quotient <= 0;
        end else if (advance) begin
            bits_left <= {bits_left[49:0], 1'b0};
            remainder <= reduced[10:0];
            quotient <= {quotient[49:0], subtract};
        end
    end
endmodule
