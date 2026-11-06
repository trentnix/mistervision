// SPDX-License-Identifier: GPL-2.0-or-later
module yc_test;
    reg clk = 0;
    always #5 clk = !clk;
    reg [39:0] phase_inc;
    reg [16:0] burst_range;
    reg pal = 0, cvbs = 0;
    reg [23:0] rgb = 0;
    reg hs = 0, vs = 0, cs = 0, de = 0;
    wire [23:0] video;
    wire hs_out, vs_out, cs_out, de_out;
    interlaced_yc dut(.*);
    reg [4:0] h_history = 0, v_history = 0, c_history = 0, d_history = 0;
    integer ticks = 0, crossings = 0, last_chroma = 128;
    task tick;
        @(negedge clk);
        h_history = {h_history[3:0], hs};
        v_history = {v_history[3:0], vs};
        c_history = {c_history[3:0], cs};
        d_history = {d_history[3:0], de};
        @(posedge clk); #1;
        ticks++;
        if (ticks > 32 && {hs_out, vs_out, cs_out, de_out} !==
            {h_history[4], v_history[4], c_history[4], d_history[4]})
            $fatal(1, "Encoder sync is not aligned: got %b expected %b phase1=%b phase4=%b", {hs_out, vs_out, cs_out, de_out}, {h_history[4], v_history[4], c_history[4], d_history[4]}, dut.encoder.phase[1].hsync, dut.encoder.phase[4].hsync);
    endtask
    // RGB stays black after sync. Only the color burst should cross the
    // blanking level on the C pin (S-Video) or the mixed Y pin (composite).
    task check_color_burst(input bit composite);
        integer blank, previous, level, rises;
        blank = composite ? 64 : 128;
        previous = blank;
        rises = 0;
        repeat (100) begin
            tick();
            level = composite ? int'(video[15:8]) : int'(video[23:16]);
            if (previous < blank && level >= blank) rises++;
            previous = level;
        end
        // The configured burst is nine cycles; allow one crossing at either edge.
        if (rises < 8 || rises > 10)
            $fatal(1, "Color burst: PAL=%0d CVBS=%0d expected 8-10 rises, got %0d", pal, composite, rises);
        if (previous != blank)
            $fatal(1, "Burst leaked into blanking: PAL=%0d CVBS=%0d level=%0d", pal, composite, previous);
    endtask

    task standard(input bit p, input bit composite, input [39:0] phase,
                  input [16:0] burst, input [39:0] expected_phase,
                  input [16:0] expected_burst, input integer expected_crossings);
        pal = p; cvbs = composite; phase_inc = phase; burst_range = burst;
        hs = 1; vs = 0; cs = 1; de = 0; rgb = 0;
        repeat(64) tick();
        if (dut.scaled_phase != expected_phase || dut.scaled_burst != expected_burst)
            $fatal(1, "Incorrect carrier or burst scaling: %d %d", dut.scaled_phase, dut.scaled_burst);
        hs = 0; cs = 0;
        check_color_burst(composite);
        rgb = 24'hff0000; de = 1;
        repeat(100) tick();
        crossings = 0;
        last_chroma = int'(video[23:16]);
        repeat(10000) begin
            tick();
            if (!composite && last_chroma < 128 && video[23:16] >= 128) crossings++;
            last_chroma = int'(video[23:16]);
            if (composite && video[23:16] != 0) $fatal(1, "CVBS drove chroma pin");
            if (!composite && video[15:8] != 76) $fatal(1, "Red luma changed");
            if (video[7:0] != 0) $fatal(1, "Unused DAC channel not black");
        end
        if (!composite && (crossings < expected_crossings - 2 || crossings > expected_crossings + 2))
            $fatal(1, "Carrier frequency: %d crossings instead of %d", crossings, expected_crossings);
        // Exercise both full-line and half-line vertical boundaries. The encoder
        // must delay every sync together without altering the incoming cadence.
        for (int line_no=0; line_no<12; line_no++) begin
            for (int pixel_no=0; pixel_no<800; pixel_no++) begin
                hs = pixel_no < 60;
                vs = (line_no>=3 && line_no<6) || (line_no==8 && pixel_no>=400);
                cs = hs ^ vs;
                de = pixel_no>=130 && !vs;
                rgb = de ? 24'hffffff : 24'h000000;
                tick();
            end
        end
        $display("PASS pal=%0d cvbs=%0d phase=%0d", p, composite, phase);
    endtask
    // Compare the register arithmetic against a wide integer reference across
    // boundary values and deterministic pseudo-random settings. Every setting
    // must publish together, including PAL/CVBS and both burst endpoints.
    task check_conversion(input [39:0] phase, input [16:0] burst,
                          input bit p, input bit composite);
        reg [63:0] expected;
        reg [16:0] expected_burst;
        phase_inc = phase; burst_range = burst; pal = p; cvbs = composite;
        expected = (64'(phase) * 1600 + 503) / 1007;
        expected_burst = {7'((32'(burst[16:10]) * 1007) / 1600),
                          10'((32'(burst[9:0]) * 1007) / 1600)};
        repeat(128) tick();
        if (dut.scaled_phase !== expected[39:0] ||
            dut.scaled_burst !== expected_burst ||
            dut.active !== {p, composite, burst, phase})
            $fatal(1, "Conversion mismatch phase=%d burst=%d", phase, burst);
    endtask
    initial begin
        standard(0,0,40'd196787592471,{7'd20,10'd70},40'd312671447819,{7'd12,10'd44},2844);
        standard(0,1,40'd196787592471,{7'd20,10'd70},40'd312671447819,{7'd12,10'd44},2844);
        standard(1,0,40'd243740768438,{7'd16,10'd56},40'd387274309335,{7'd10,10'd35},3522);
        standard(1,1,40'd243740768438,{7'd16,10'd56},40'd387274309335,{7'd10,10'd35},3522);
        standard(1,0,40'd196571293545,{7'd20,10'd70},40'd312327775245,{7'd12,10'd44},2841);
        // A new SPI setting during conversion must replace the pending one.
        phase_inc = 40'd196787592471;
        repeat(12) tick();
        phase_inc = 40'd243740768438;
        burst_range = {7'd16,10'd56};
        cvbs = 1;
        repeat(128) tick();
        if (dut.scaled_phase != 40'd387274309335 || !dut.active[57])
            $fatal(1, "Changed configuration was not published coherently");
        $display("PASS configuration change during conversion");
        check_conversion(40'd0, 17'd0, 0, 0);
        check_conversion(40'd1, 17'd1, 1, 1);
        check_conversion(40'hffffffffff, 17'h1ffff, 0, 1);
        for (int trial=0; trial<256; trial++)
            check_conversion({8'($random),32'($random)}, 17'($random),
                             trial[0], trial[1]);
        $display("PASS boundary and randomized coherent conversions");
        $finish;
    end
endmodule
